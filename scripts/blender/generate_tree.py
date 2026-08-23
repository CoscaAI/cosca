#!/usr/bin/env python3
"""
Cosca Procedural Tree Generator

Generates procedural trees using L-systems with species-specific parameters.

Usage:
  blender --background --factory-startup --python generate_tree.py -- \
    --species oak --age mature --seed 42 --output /tmp/tree.glb

Species: oak, pine, birch, palm, fruit_tree
Ages: seedling, young, mature, old, ancient
"""

import bpy
import bmesh
import sys
import os
import json
import math
import random
from mathutils import Vector, Matrix, Euler

# ──────────────────────────────────────────────────────────────
# Tree Species Parameters
# ──────────────────────────────────────────────────────────────

SPECIES = {
    "oak": {
        "height_range": (8, 15),
        "trunk_radius": (0.3, 0.5),
        "trunk_taper": 0.7,
        "branch_levels": 4,
        "branch_angle": (25, 45),
        "branch_density": (0.6, 0.8),
        "crown_shape": "rounded",
        "crown_radius": (4, 7),
        "leaf_size": (0.1, 0.2),
        "leaf_density": 0.7,
        "growth_direction": 0.7,
        "bark_color": (0.25, 0.15, 0.08),
        "leaf_color": (0.1, 0.35, 0.05),
        "branch_curvature": 0.3,
        "branch_length_factor": 0.65,
    },
    "pine": {
        "height_range": (10, 20),
        "trunk_radius": (0.2, 0.4),
        "trunk_taper": 0.5,
        "branch_levels": 5,
        "branch_angle": (60, 80),
        "branch_density": (0.4, 0.6),
        "crown_shape": "conical",
        "crown_radius": (2, 4),
        "leaf_size": (0.05, 0.1),
        "leaf_density": 0.9,
        "growth_direction": 0.9,
        "bark_color": (0.3, 0.18, 0.1),
        "leaf_color": (0.05, 0.25, 0.08),
        "branch_curvature": 0.1,
        "branch_length_factor": 0.55,
    },
    "birch": {
        "height_range": (8, 12),
        "trunk_radius": (0.15, 0.25),
        "trunk_taper": 0.6,
        "branch_levels": 4,
        "branch_angle": (30, 50),
        "branch_density": (0.7, 0.9),
        "crown_shape": "oval",
        "crown_radius": (3, 5),
        "leaf_size": (0.08, 0.15),
        "leaf_density": 0.8,
        "growth_direction": 0.6,
        "bark_color": (0.85, 0.82, 0.78),
        "leaf_color": (0.15, 0.4, 0.1),
        "branch_curvature": 0.4,
        "branch_length_factor": 0.7,
    },
    "palm": {
        "height_range": (5, 15),
        "trunk_radius": (0.15, 0.3),
        "trunk_taper": 0.2,
        "branch_levels": 1,
        "branch_angle": (80, 90),
        "branch_density": (0.3, 0.5),
        "crown_shape": "fan",
        "crown_radius": (3, 6),
        "leaf_size": (0.5, 1.0),
        "leaf_density": 0.5,
        "growth_direction": 0.95,
        "bark_color": (0.35, 0.25, 0.15),
        "leaf_color": (0.1, 0.3, 0.05),
        "branch_curvature": 0.05,
        "branch_length_factor": 0.8,
    },
    "fruit_tree": {
        "height_range": (4, 7),
        "trunk_radius": (0.15, 0.25),
        "trunk_taper": 0.65,
        "branch_levels": 3,
        "branch_angle": (35, 55),
        "branch_density": (0.7, 0.9),
        "crown_shape": "rounded",
        "crown_radius": (2.5, 4),
        "leaf_size": (0.1, 0.18),
        "leaf_density": 0.75,
        "growth_direction": 0.5,
        "bark_color": (0.3, 0.2, 0.12),
        "leaf_color": (0.12, 0.38, 0.08),
        "branch_curvature": 0.35,
        "branch_length_factor": 0.6,
    },
}

# Age multipliers
AGE_MULTIPLIERS = {
    "seedling": {"height": 0.15, "radius": 0.3, "branches": 0.2, "crown": 0.2},
    "young": {"height": 0.4, "radius": 0.5, "branches": 0.5, "crown": 0.5},
    "mature": {"height": 1.0, "radius": 1.0, "branches": 1.0, "crown": 1.0},
    "old": {"height": 0.9, "radius": 1.3, "branches": 1.2, "crown": 0.8},
    "ancient": {"height": 0.7, "radius": 1.6, "branches": 1.4, "crown": 0.6},
}


# ──────────────────────────────────────────────────────────────
# L-System
# ──────────────────────────────────────────────────────────────

class LSystem:
    """Simple L-System for tree generation."""

    def __init__(self, axiom, rules, iterations):
        self.axiom = axiom
        self.rules = rules
        self.iterations = iterations

    def generate(self):
        current = self.axiom
        for _ in range(self.iterations):
            next_str = ""
            for char in current:
                if char in self.rules:
                    next_str += self.rules[char]
                else:
                    next_str += char
            current = next_str
        return current


def tree_lsystem(species_params, seed):
    """Generate L-system string for tree."""
    random.seed(seed)

    branch_angle = random.uniform(*species_params["branch_angle"])
    iterations = min(species_params["branch_levels"], 4)

    # Modified L-system for trees
    # F = forward, + = turn left, - = turn right, [ = push, ] = pop
    axiom = "X"
    rules = {
        "X": f"F[-X][+X]F[--X][++X]X",
        "F": "FF",
    }

    # Adjust based on species
    if species_params["crown_shape"] == "conical":
        rules["X"] = f"F[-X][+X]F[-X][+X]X"
    elif species_params["crown_shape"] == "fan":
        rules["X"] = f"F[+X]F[-X]X"

    system = LSystem(axiom, rules, iterations)
    return system.generate()


# ──────────────────────────────────────────────────────────────
# Mesh Generation
# ──────────────────────────────────────────────────────────────

def create_branch_mesh(name, points, radii, material):
    """Create a branch mesh from a series of points with radii."""
    mesh = bpy.data.meshes.new(name)
    obj = bpy.data.objects.new(name, mesh)
    bm = bmesh.new()

    # Create tube along points
    for i in range(len(points) - 1):
        p1 = points[i]
        p2 = points[i + 1]
        r1 = radii[i]
        r2 = radii[i + 1]

        # Create ring at p1
        ring1 = []
        segments = 8
        direction = (p2 - p1).normalized()
        up = Vector((0, 0, 1))
        if abs(direction.dot(up)) > 0.9:
            up = Vector((1, 0, 0))
        right = direction.cross(up).normalized()
        up = right.cross(direction).normalized()

        for j in range(segments):
            angle = (j / segments) * 2 * math.pi
            offset = right * math.cos(angle) * r1 + up * math.sin(angle) * r1
            ring1.append(bm.verts.new(p1 + offset))

        # Create ring at p2
        ring2 = []
        for j in range(segments):
            angle = (j / segments) * 2 * math.pi
            offset = right * math.cos(angle) * r2 + up * math.sin(angle) * r2
            ring2.append(bm.verts.new(p2 + offset))

        # Connect rings
        for j in range(segments):
            j_next = (j + 1) % segments
            try:
                bm.faces.new([ring1[j], ring1[j_next], ring2[j_next], ring2[j]])
            except:
                pass

    bm.to_mesh(mesh)
    bm.free()

    obj.data.materials.append(material)
    return obj


def create_foliage_cluster(name, position, radius, density, material, seed):
    """Create a foliage cluster as a displaced icosphere."""
    random.seed(seed)

    # Create icosphere
    bpy.ops.mesh.primitive_ico_sphere_add(
        subdivisions=2,
        radius=radius,
        location=position
    )
    cluster = bpy.context.active_object
    cluster.name = name

    # Displace vertices for organic look
    mesh = cluster.data
    for vert in mesh.vertices:
        # Random displacement
        displacement = random.uniform(-0.3, 0.3) * radius
        vert.co += Vector((
            random.uniform(-1, 1) * displacement,
            random.uniform(-1, 1) * displacement,
            random.uniform(-1, 1) * displacement
        ))

    # Apply material
    cluster.data.materials.append(material)

    return cluster


# ──────────────────────────────────────────────────────────────
# Tree Generator
# ──────────────────────────────────────────────────────────────

def generate_tree(species_name, age, seed, scale=1.0):
    """
    Generate a procedural tree.

    Returns list of objects (trunk, branches, foliage).
    """
    species = SPECIES[species_name]
    age_mult = AGE_MULTIPLIERS[age]

    random.seed(seed)

    # Calculate actual dimensions
    height = random.uniform(*species["height_range"]) * age_mult["height"] * scale
    trunk_radius = random.uniform(*species["trunk_radius"]) * age_mult["radius"] * scale
    crown_radius = random.uniform(*species["crown_radius"]) * age_mult["crown"] * scale

    print(f"Generating {species_name} tree ({age}): height={height:.2f}, trunk_r={trunk_radius:.2f}, crown_r={crown_radius:.2f}")

    # Clear scene
    bpy.ops.wm.read_factory_settings(use_empty=True)

    # Create materials
    # Bark material
    bark_mat = bpy.data.materials.new(name="BarkMaterial")
    bark_mat.use_nodes = True
    bsdf = bark_mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (*species["bark_color"], 1)
    bsdf.inputs['Roughness'].default_value = 0.9
    bsdf.inputs['Normal'].default_value = (0, 0, 0)

    # Leaf material
    leaf_mat = bpy.data.materials.new(name="LeafMaterial")
    leaf_mat.use_nodes = True
    bsdf = leaf_mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (*species["leaf_color"], 1)
    bsdf.inputs['Roughness'].default_value = 0.6
    bsdf.inputs['Alpha'].default_value = 0.9

    objects = []

    # ── Generate trunk using L-system ──
    lsystem_str = tree_lsystem(species, seed)

    # Parse L-system and generate geometry
    trunk_points = [Vector((0, 0, 0))]
    trunk_radii = [trunk_radius]
    current_pos = Vector((0, 0, 0))
    current_dir = Vector((0, 0, 1))
    segment_length = height / 20
    stack = []
    branch_points = []

    # Track branch tips for foliage
    branch_tips = []

    for char in lsystem_str:
        if char == 'F':
            # Move forward
            next_pos = current_pos + current_dir * segment_length
            trunk_points.append(next_pos)
            trunk_radii.append(trunk_radii[-1] * 0.95)  # Taper
            current_pos = next_pos
        elif char == '+':
            # Turn right
            angle = species["branch_angle"][0] * math.pi / 180
            current_dir = rotate_vector(current_dir, angle, 'x')
        elif char == '-':
            # Turn left
            angle = species["branch_angle"][1] * math.pi / 180
            current_dir = rotate_vector(current_dir, -angle, 'x')
        elif char == '[':
            # Push state
            stack.append((current_pos.copy(), current_dir.copy(), len(trunk_points) - 1))
        elif char == ']':
            # Pop state and start branch
            if stack:
                pos, direction, idx = stack.pop()
                branch_points.append((pos, direction))
                branch_tips.append(pos + direction * segment_length * 3)
                current_pos = pos
                current_dir = direction

    # Create trunk mesh
    if len(trunk_points) > 1:
        trunk = create_branch_mesh("Trunk", trunk_points, trunk_radii, bark_mat)
        objects.append(trunk)

    # ── Generate branches ──
    for i, (start, direction) in enumerate(branch_points[:int(species["branch_levels"] * 5)]):
        branch_len = height * 0.3 * random.uniform(0.5, 1.0)
        branch_radius = trunk_radius * 0.3 * random.uniform(0.5, 1.0)

        points = [start]
        radii = [branch_radius]
        pos = start
        dir = direction

        for j in range(5):
            pos = pos + dir * branch_len / 5
            dir = rotate_vector(dir, random.uniform(-0.3, 0.3), 'x')
            dir = rotate_vector(dir, random.uniform(-0.3, 0.3), 'y')
            points.append(pos)
            radii.append(branch_radius * (1 - j / 5))

        branch_tips.append(pos)

        branch = create_branch_mesh(f"Branch_{i}", points, radii, bark_mat)
        objects.append(branch)

    # ── Generate foliage clusters ──
    num_clusters = int(species["leaf_density"] * 20 * age_mult["branches"])

    for i in range(num_clusters):
        # Position clusters around branch tips
        if branch_tips:
            base_pos = random.choice(branch_tips)
            offset = Vector((
                random.uniform(-crown_radius * 0.5, crown_radius * 0.5),
                random.uniform(-crown_radius * 0.5, crown_radius * 0.5),
                random.uniform(0, crown_radius * 0.5)
            ))
            pos = base_pos + offset
        else:
            pos = Vector((
                random.uniform(-crown_radius, crown_radius),
                random.uniform(-crown_radius, crown_radius),
                height * 0.6 + random.uniform(0, crown_radius)
            ))

        cluster_radius = crown_radius * random.uniform(0.2, 0.5)
        cluster = create_foliage_cluster(
            f"Foliage_{i}",
            pos,
            cluster_radius,
            species["leaf_density"],
            leaf_mat,
            seed + i
        )
        objects.append(cluster)

    print(f"Generated {len(objects)} objects")
    return objects


def rotate_vector(v, angle, axis):
    """Rotate vector around axis by angle (radians)."""
    cos_a = math.cos(angle)
    sin_a = math.sin(angle)

    if axis == 'x':
        return Vector((
            v.x,
            v.y * cos_a - v.z * sin_a,
            v.y * sin_a + v.z * cos_a
        ))
    elif axis == 'y':
        return Vector((
            v.x * cos_a + v.z * sin_a,
            v.y,
            -v.x * sin_a + v.z * cos_a
        ))
    else:  # z
        return Vector((
            v.x * cos_a - v.y * sin_a,
            v.x * sin_a + v.y * cos_a,
            v.z
        ))


# ──────────────────────────────────────────────────────────────
# Export
# ──────────────────────────────────────────────────────────────

def export_tree(output_path, objects):
    """Export tree to GLB."""
    # Make sure all objects are in the scene and visible
    for obj in objects:
        obj.hide_viewport = False
        obj.hide_render = False
        # Link to scene if not already
        if obj.name not in bpy.context.scene.objects:
            bpy.context.scene.collection.objects.link(obj)

    # Select all objects
    bpy.ops.object.select_all(action='DESELECT')
    for obj in objects:
        obj.select_set(True)
        bpy.context.view_layer.objects.active = obj

    # Export as GLB
    bpy.ops.export_scene.gltf(
        filepath=output_path,
        export_format='GLB',
        use_selection=True,
        export_apply=True,
        export_materials='EXPORT',
    )

    # Compute stats
    total_verts = 0
    total_faces = 0
    for obj in objects:
        if obj.type == 'MESH':
            total_verts += len(obj.data.vertices)
            total_faces += len(obj.data.polygons)

    return {
        "objects": len(objects),
        "vertices": total_verts,
        "faces": total_faces,
    }


# ──────────────────────────────────────────────────────────────
# Main
# ──────────────────────────────────────────────────────────────

def main():
    # Parse arguments
    argv = sys.argv
    if "--" not in argv:
        argv = []
    else:
        argv = argv[argv.index("--") + 1:]

    import argparse
    parser = argparse.ArgumentParser(description="Cosca Procedural Tree Generator")
    parser.add_argument("--species", default="oak", choices=SPECIES.keys())
    parser.add_argument("--age", default="mature", choices=AGE_MULTIPLIERS.keys())
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--scale", type=float, default=1.0)
    parser.add_argument("--output", required=True)

    args = parser.parse_args(argv)

    # Generate tree
    print(f"\n=== Generating {args.species} tree ({args.age}) ===")
    objects = generate_tree(args.species, args.age, args.seed, args.scale)

    # Export
    stats = export_tree(args.output, objects)
    print(f"\n=== Exported to {args.output} ===")
    print(f"Objects: {stats['objects']}, Vertices: {stats['vertices']}, Faces: {stats['faces']}")

    # Output metadata
    metadata = {
        "success": True,
        "species": args.species,
        "age": args.age,
        "seed": args.seed,
        "output": args.output,
        "stats": stats,
    }
    print(json.dumps(metadata))


if __name__ == "__main__":
    main()
