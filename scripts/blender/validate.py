#!/usr/bin/env python3
"""
Blender Asset Validator for Cosca Living World.

Usage:
  blender --background --factory-startup --python validate.py -- --input /tmp/asset.glb

Validates an asset file and outputs a JSON report.
"""

import bpy
import sys
import argparse
import json
import os


def validate_mesh(obj):
    """Validate a single mesh object."""
    issues = []
    
    mesh = obj.data
    
    # Check vertices
    if len(mesh.vertices) == 0:
        issues.append(f"{obj.name}: empty mesh (0 vertices)")
    
    # Check UVs
    if len(mesh.uv_layers) == 0:
        issues.append(f"{obj.name}: no UV layers")
    
    # Check normals (auto-calculated in Blender 4.1+, validate normal presence via polygons)
    if len(mesh.polygons) > 0 and not mesh.has_custom_normals:
        # Check that at least one normal is valid
        for poly in mesh.polygons:
            if poly.normal.length < 0.5:
                issues.append(f"{obj.name}: degenerate normals on face {poly.index}")
                break
    
    # Check for n-gons (faces with > 4 vertices)
    for i, poly in enumerate(mesh.polygons):
        if len(poly.vertices) > 4:
            issues.append(f"{obj.name}: n-gon face {i} with {len(poly.vertices)} vertices")
    
    # Check bounding box
    from mathutils import Vector
    bbox = [obj.matrix_world @ Vector(corner) for corner in obj.bound_box]
    min_co = [min(v[i] for v in bbox) for i in range(3)]
    max_co = [max(v[i] for v in bbox) for i in range(3)]
    size = [max_co[i] - min_co[i] for i in range(3)]
    
    if max(size) > 1000:
        issues.append(f"{obj.name}: bounding box too large: {size}")
    
    if max(size) < 0.001:
        issues.append(f"{obj.name}: bounding box too small: {size}")
    
    # Check for degenerate faces
    for i, poly in enumerate(mesh.polygons):
        if poly.area < 0.0001:
            issues.append(f"{obj.name}: degenerate face {i}")
    
    return {
        'name': obj.name,
        'vertices': len(mesh.vertices),
        'faces': len(mesh.polygons),
        'uv_layers': len(mesh.uv_layers),
        'has_normals': len(mesh.polygons) > 0,
        'materials': len(obj.data.materials),
        'bounding_box': {
            'min': min_co,
            'max': max_co,
            'size': size,
        },
        'issues': issues,
    }


def validate_material(mat):
    """Validate a material."""
    issues = []
    
    if not mat.use_nodes:
        issues.append(f"{mat.name}: not using nodes")
        return {'name': mat.name, 'valid': False, 'issues': issues}
    
    # Check for Principled BSDF
    has_bsdf = False
    for node in mat.node_tree.nodes:
        if node.type == 'BSDF_PRINCIPLED':
            has_bsdf = True
            break
    
    if not has_bsdf:
        issues.append(f"{mat.name}: no Principled BSDF node")
    
    return {
        'name': mat.name,
        'valid': len(issues) == 0,
        'issues': issues,
    }


def validate_asset(filepath):
    """Validate an asset file."""
    all_issues = []
    mesh_stats = []
    material_stats = []
    
    # Import file
    ext = os.path.splitext(filepath)[1].lower()
    
    if ext == '.glb' or ext == '.gltf':
        bpy.ops.import_scene.gltf(filepath=filepath)
    elif ext == '.fbx':
        bpy.ops.import_scene.fbx(filepath=filepath)
    elif ext == '.blend':
        bpy.ops.wm.open_mainfile(filepath=filepath)
    else:
        return {
            'valid': False,
            'issues': [f"Unsupported format: {ext}"],
            'stats': {},
        }
    
    # Validate all mesh objects
    for obj in bpy.context.scene.objects:
        if obj.type == 'MESH':
            result = validate_mesh(obj)
            mesh_stats.append(result)
            all_issues.extend(result['issues'])
    
    # Validate all materials
    for mat in bpy.data.materials:
        result = validate_material(mat)
        material_stats.append(result)
        all_issues.extend(result['issues'])
    
    # Overall stats — in Cosca AssetMetadata format
    total_vertices = sum(m['vertices'] for m in mesh_stats)
    total_faces = sum(m['faces'] for m in mesh_stats)

    stats = {
        'vertices': total_vertices,
        'faces': total_faces,
        'materials': [m['name'] for m in material_stats],
        'textures': [],
        'lod_levels': 1,
        'has_collision': False,
        'object_count': len([o for o in bpy.context.scene.objects if o.type == 'MESH']),
        'bounding_box': None,
        'meshes': mesh_stats,
        'material_details': material_stats,
    }
    
    return {
        'valid': len(all_issues) == 0,
        'issues': all_issues,
        'stats': stats,
    }


def main():
    # Parse arguments after "--"
    argv = sys.argv
    if "--" not in argv:
        argv = []
    else:
        argv = argv[argv.index("--") + 1:]
    
    parser = argparse.ArgumentParser(description="Validate Blender asset")
    parser.add_argument("--input", required=True, help="Path to asset file")
    
    args = parser.parse_args(argv)
    
    if not os.path.exists(args.input):
        result = {
            'valid': False,
            'issues': [f"File not found: {args.input}"],
            'stats': {},
        }
    else:
        # Clear scene first
        bpy.ops.wm.read_factory_settings(use_empty=True)
        result = validate_asset(args.input)
    
    # Output as JSON
    print(json.dumps(result))


if __name__ == "__main__":
    main()
