#!/usr/bin/env python3
"""
Cosca VS#1 — Procedural Tree Generator (Improved)

Generates organic trees with:
- Natural trunk with taper and slight curves
- Progressive branching with diameter reduction
- Foliage clusters (not spheres)
- 2 LOD levels (full + simple)
- Proper materials (bark + leaf)

Usage:
  blender --background --factory-startup --python generate_vs1_tree.py -- \
    --species oak --seed 42 --output /tmp/tree.glb
"""

import bpy
import bmesh
import sys
import json
import math
import random
from mathutils import Vector

# ──────────────────────────────────────────────────────────────
# Species Parameters
# ──────────────────────────────────────────────────────────────

SPECIES = {
    "oak": {
        "height": (10, 14),
        "trunk_radius": (0.35, 0.5),
        "trunk_segments": 12,
        "trunk_height_ratio": 0.4,
        "branch_levels": 3,
        "branch_angle": (20, 40),
        "branch_length_factor": 0.6,
        "branch_radius_factor": 0.35,
        "branch_curvature": 0.2,
        "crown_radius": (4, 6),
        "foliage_clusters": (12, 18),
        "foliage_radius": (0.8, 1.5),
        "bark_color": (0.22, 0.13, 0.07),
        "leaf_color": (0.08, 0.32, 0.04),
    },
    "pine": {
        "height": (12, 18),
        "trunk_radius": (0.2, 0.35),
        "trunk_segments": 8,
        "trunk_height_ratio": 0.5,
        "branch_levels": 4,
        "branch_angle": (50, 70),
        "branch_length_factor": 0.45,
        "branch_radius_factor": 0.3,
        "branch_curvature": 0.1,
        "crown_radius": (2, 3.5),
        "foliage_clusters": (8, 12),
        "foliage_radius": (0.5, 1.0),
        "bark_color": (0.28, 0.16, 0.09),
        "leaf_color": (0.04, 0.22, 0.06),
    },
    "birch": {
        "height": (8, 12),
        "trunk_radius": (0.12, 0.2),
        "trunk_segments": 8,
        "trunk_height_ratio": 0.45,
        "branch_levels": 3,
        "branch_angle": (25, 45),
        "branch_length_factor": 0.65,
        "branch_radius_factor": 0.3,
        "branch_curvature": 0.35,
        "crown_radius": (3, 5),
        "foliage_clusters": (15, 22),
        "foliage_radius": (0.6, 1.2),
        "bark_color": (0.82, 0.78, 0.74),
        "leaf_color": (0.12, 0.38, 0.08),
    },
}


# ──────────────────────────────────────────────────────────────
# Geometry Helpers
# ──────────────────────────────────────────────────────────────

def lerp(a, b, t):
    return a + (b - a) * t


def clamp(x, lo, hi):
    return max(lo, min(hi, x))


def create_tube(name, points, radii, material, segments=8):
    """Create a tube mesh along a path with varying radius."""
    mesh = bpy.data.meshes.new(name)
    obj = bpy.data.objects.new(name, mesh)
    bm = bmesh.new()

    rings = []
    for idx, (pt, r) in enumerate(zip(points, radii)):
        # Compute local frame
        if idx < len(points) - 1:
            fwd = (points[idx + 1] - pt).normalized()
        else:
            fwd = (points[idx] - points[idx - 1]).normalized()

        up = Vector((0, 0, 1))
        if abs(fwd.dot(up)) > 0.9:
            up = Vector((1, 0, 0))
        right = fwd.cross(up).normalized()
        up = right.cross(fwd).normalized()

        ring = []
        for j in range(segments):
            angle = (j / segments) * 2 * math.pi
            offset = right * math.cos(angle) * r + up * math.sin(angle) * r
            ring.append(bm.verts.new(pt + offset))
        rings.append(ring)

    # Connect rings
    for i in range(len(rings) - 1):
        for j in range(segments):
            j_next = (j + 1) % segments
            try:
                bm.faces.new([rings[i][j], rings[i][j_next],
                              rings[i + 1][j_next], rings[i + 1][j]])
            except:
                pass

    bm.to_mesh(mesh)
    bm.free()
    obj.data.materials.append(material)
    return obj


def create_foliage_cluster(name, position, radius, material, seed):
    """Create an organic foliage cluster."""
    random.seed(seed)

    bpy.ops.mesh.primitive_ico_sphere_add(
        subdivisions=2, radius=radius, location=position
    )
    cluster = bpy.context.active_object
    cluster.name = name

    # Deform for organic look
    mesh = cluster.data
    for vert in mesh.vertices:
        noise = Vector((
            random.uniform(-0.4, 0.4),
            random.uniform(-0.4, 0.4),
            random.uniform(-0.3, 0.5),
        )) * radius * 0.3
        vert.co += noise

    cluster.data.materials.append(material)
    return cluster


# ──────────────────────────────────────────────────────────────
# Tree Generator
# ──────────────────────────────────────────────────────────────

def generate_oak(species_name, seed, scale=1.0):
    """Generate an oak-style tree with organic structure."""
    sp = SPECIES[species_name]
    random.seed(seed)

    height = random.uniform(*sp["height"]) * scale
    trunk_r = random.uniform(*sp["trunk_radius"]) * scale
    crown_r = random.uniform(*sp["crown_radius"]) * scale
    trunk_h = height * sp["trunk_height_ratio"]

    # Materials
    bark = bpy.data.materials.new("Bark")
    bark.use_nodes = True
    bsdf = bark.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (*sp["bark_color"], 1)
    bsdf.inputs['Roughness'].default_value = 0.92

    leaf = bpy.data.materials.new("Leaf")
    leaf.use_nodes = True
    bsdf = leaf.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (*sp["leaf_color"], 1)
    bsdf.inputs['Roughness'].default_value = 0.55
    bsdf.inputs['Alpha'].default_value = 0.92

    objects = []

    # ── TRUNK ──
    # Main trunk with slight curves and taper
    trunk_pts = []
    trunk_radii = []
    segments = sp["trunk_segments"]

    for i in range(segments + 1):
        t = i / segments
        z = t * trunk_h

        # Slight random offset for natural look
        x_off = random.uniform(-0.1, 0.1) * trunk_h * 0.1
        y_off = random.uniform(-0.1, 0.1) * trunk_h * 0.1

        # Taper: wider at base, narrower at top
        r = lerp(trunk_r, trunk_r * 0.3, t ** 0.7)

        # Flare at base
        if t < 0.1:
            r *= lerp(1.3, 1.0, t / 0.1)

        trunk_pts.append(Vector((x_off, y_off, z)))
        trunk_radii.append(r)

    trunk = create_tube("Trunk", trunk_pts, trunk_radii, bark, segments=10)
    objects.append(trunk)

    # ── BRANCHES ──
    branch_tips = []
    num_branches = sp["branch_levels"] * 4

    for b in range(num_branches):
        # Start point on trunk
        t_start = random.uniform(0.3, 0.85)
        start_idx = int(t_start * segments)
        start_idx = clamp(start_idx, 0, len(trunk_pts) - 1)
        start = trunk_pts[start_idx].copy()

        # Direction: outward and slightly up
        angle_h = random.uniform(0, 2 * math.pi)
        angle_v = random.uniform(*sp["branch_angle"]) * math.pi / 180

        direction = Vector((
            math.cos(angle_h) * math.cos(angle_v),
            math.sin(angle_h) * math.cos(angle_v),
            math.sin(angle_v) * 0.5 + 0.3,
        )).normalized()

        # Branch length decreases with height
        length = height * sp["branch_length_factor"] * random.uniform(0.5, 1.0) * (1 - t_start * 0.5)

        # Generate branch points
        branch_pts = [start]
        branch_radii = [trunk_radii[start_idx] * sp["branch_radius_factor"]]
        pos = start
        dir = direction

        num_segs = 5
        for s in range(num_segs):
            t = (s + 1) / num_segs
            seg_len = length / num_segs

            # Slight curvature
            dir = dir.normalized()
            dir.x += random.uniform(-sp["branch_curvature"], sp["branch_curvature"])
            dir.y += random.uniform(-sp["branch_curvature"], sp["branch_curvature"])
            dir.z -= 0.05  # slight droop
            dir = dir.normalized()

            pos = pos + dir * seg_len
            r = lerp(branch_radii[0], branch_radii[0] * 0.1, t)
            branch_pts.append(pos)
            branch_radii.append(r)

        branch_tips.append(pos)

        branch = create_tube(f"Branch_{b}", branch_pts, branch_radii, bark, segments=6)
        objects.append(branch)

    # ── FOLIAGE CLUSTERS ──
    num_clusters = random.randint(*sp["foliage_clusters"])

    for i in range(num_clusters):
        # Position near branch tips or in crown volume
        if branch_tips and random.random() > 0.3:
            base = random.choice(branch_tips)
            offset = Vector((
                random.uniform(-crown_r * 0.4, crown_r * 0.4),
                random.uniform(-crown_r * 0.4, crown_r * 0.4),
                random.uniform(-0.5, crown_r * 0.3),
            ))
            pos = base + offset
        else:
            # Random in crown volume
            angle = random.uniform(0, 2 * math.pi)
            r = random.uniform(0, crown_r * 0.8)
            z = trunk_h + random.uniform(0, height * 0.4)
            pos = Vector((math.cos(angle) * r, math.sin(angle) * r, z))

        cluster_r = random.uniform(*sp["foliage_radius"]) * scale
        cluster = create_foliage_cluster(
            f"Foliage_{i}", pos, cluster_r, leaf, seed + i + 100
        )
        objects.append(cluster)

    return objects


def generate_simple_version(species_name, seed, scale=1.0):
    """Generate simplified version for LOD1."""
    sp = SPECIES[species_name]
    random.seed(seed + 1000)

    height = random.uniform(*sp["height"]) * scale
    trunk_r = random.uniform(*sp["trunk_radius"]) * scale
    crown_r = random.uniform(*sp["crown_radius"]) * scale

    bark = bpy.data.materials.new("Bark_Simple")
    bark.use_nodes = True
    bsdf = bark.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (*sp["bark_color"], 1)
    bsdf.inputs['Roughness'].default_value = 0.9

    leaf = bpy.data.materials.new("Leaf_Simple")
    leaf.use_nodes = True
    bsdf = leaf.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (*sp["leaf_color"], 1)
    bsdf.inputs['Roughness'].default_value = 0.6
    bsdf.inputs['Alpha'].default_value = 0.9

    objects = []

    # Simple trunk (fewer segments)
    trunk_h = height * sp["trunk_height_ratio"]
    trunk_pts = [Vector((0, 0, 0)), Vector((0, 0, trunk_h))]
    trunk_radii = [trunk_r, trunk_r * 0.3]
    trunk = create_tube("Trunk_Simple", trunk_pts, trunk_radii, bark, segments=6)
    objects.append(trunk)

    # Simple crown (fewer clusters)
    num_clusters_simple = max(5, random.randint(*sp["foliage_clusters"]) // 3)
    for i in range(num_clusters_simple):
        angle = random.uniform(0, 2 * math.pi)
        r = random.uniform(0, crown_r * 0.7)
        z = trunk_h + random.uniform(0, height * 0.35)
        pos = Vector((math.cos(angle) * r, math.sin(angle) * r, z))
        cluster_r = random.uniform(1.0, 2.0) * scale
        cluster = create_foliage_cluster(
            f"Crown_{i}", pos, cluster_r, leaf, seed + i + 200
        )
        objects.append(cluster)

    return objects


# ──────────────────────────────────────────────────────────────
# Export
# ──────────────────────────────────────────────────────────────

def export_to_glb(output_path, objects):
    """Export objects to GLB as a single combined mesh."""
    for obj in objects:
        if obj.name not in bpy.context.scene.objects:
            bpy.context.scene.collection.objects.link(obj)
        obj.hide_viewport = False
        obj.hide_render = False

    # Select all mesh objects
    bpy.ops.object.select_all(action='DESELECT')
    mesh_objects = [o for o in objects if o.type == 'MESH']
    for obj in mesh_objects:
        obj.select_set(True)
        bpy.context.view_layer.objects.active = obj

    # Join all meshes into one
    if len(mesh_objects) > 1:
        bpy.ops.object.join()
    
    # Get the joined object
    joined = bpy.context.active_object
    if joined:
        joined.name = "Tree_Combined"
    
    # Recalculate normals
    bpy.ops.object.mode_set(mode='EDIT')
    bpy.ops.mesh.normals_make_consistent(inside=False)
    bpy.ops.object.mode_set(mode='OBJECT')

    # Export single mesh
    bpy.ops.export_scene.gltf(
        filepath=output_path,
        export_format='GLB',
        use_selection=True,
        export_apply=True,
        export_materials='EXPORT',
    )

    total_v = len(joined.data.vertices) if joined else 0
    total_f = len(joined.data.polygons) if joined else 0
    return {"objects": 1, "vertices": total_v, "faces": total_f}


# ──────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────

def main():
    argv = sys.argv
    if "--" not in argv:
        argv = []
    else:
        argv = argv[argv.index("--") + 1:]

    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--species", default="oak", choices=SPECIES.keys())
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--scale", type=float, default=1.0)
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)

    bpy.ops.wm.read_factory_settings(use_empty=True)

    print(f"\n=== VS#1: Generating {args.species} tree (seed={args.seed}) ===")

    # Full detail version
    objects = generate_oak(args.species, args.seed, args.scale)
    stats = export_to_glb(args.output, objects)
    print(f"LOD0: {stats['objects']} objects, {stats['vertices']} verts, {stats['faces']} faces")

    # Simple version for LOD1
    simple_output = args.output.replace(".glb", "_simple.glb")
    simple_objects = generate_simple_version(args.species, args.seed, args.scale)
    simple_stats = export_to_glb(simple_output, simple_objects)
    print(f"LOD1: {simple_stats['objects']} objects, {simple_stats['vertices']} verts, {simple_stats['faces']} faces")

    print(json.dumps({
        "success": True,
        "species": args.species,
        "seed": args.seed,
        "lod0": {"output": args.output, **stats},
        "lod1": {"output": simple_output, **simple_stats},
    }))


if __name__ == "__main__":
    main()
