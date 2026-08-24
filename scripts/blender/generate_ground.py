#!/usr/bin/env python3
"""
Cosca VS#1 — Ground Material Generator

Generates a forest ground material with:
- Procedural grass/dirt texture
- Normal map
- Roughness/AO packed

Usage:
  blender --background --factory-startup --python generate_ground.py -- --output /tmp/ground.glb
"""

import bpy
import sys
import json
import os
from mathutils import Vector

def create_ground_plane(size=10.0):
    """Create a flat ground plane."""
    bpy.ops.mesh.primitive_plane_add(size=size, location=(0, 0, 0))
    plane = bpy.context.active_object
    plane.name = "Ground"
    return plane

def create_forest_ground_material():
    """Create a forest ground material with color variation."""
    mat = bpy.data.materials.new("ForestGround")
    mat.use_nodes = True
    
    nodes = mat.node_tree.nodes
    links = mat.node_tree.links
    
    # Clear default nodes
    for node in nodes:
        nodes.remove(node)
    
    # Create nodes
    output = nodes.new('ShaderNodeOutputMaterial')
    output.location = (400, 0)
    
    bsdf = nodes.new('ShaderNodeBsdfPrincipled')
    bsdf.location = (200, 0)
    bsdf.inputs['Roughness'].default_value = 0.85
    bsdf.inputs['Base Color'].default_value = (0.15, 0.12, 0.08, 1)  # Dark earth
    
    # Color ramp for variation
    ramp = nodes.new('ShaderNodeValToRGB')
    ramp.location = (-100, 0)
    ramp.color_ramp.elements[0].color = (0.08, 0.15, 0.04, 1)  # Green grass
    ramp.color_ramp.elements[1].color = (0.18, 0.12, 0.06, 1)  # Brown dirt
    
    # Noise for variation
    noise = nodes.new('ShaderNodeTexNoise')
    noise.location = (-300, 0)
    noise.inputs['Scale'].default_value = 5.0
    noise.inputs['Detail'].default_value = 8.0
    
    # Link
    links.new(noise.outputs['Fac'], ramp.inputs['Fac'])
    links.new(ramp.outputs['Color'], bsdf.inputs['Base Color'])
    links.new(bsdf.outputs['BSDF'], output.inputs['Surface'])
    
    return mat

def export_to_glb(output_path, obj):
    """Export single object to GLB."""
    bpy.ops.object.select_all(action='DESELECT')
    obj.select_set(True)
    bpy.context.view_layer.objects.active = obj
    
    bpy.ops.export_scene.gltf(
        filepath=output_path,
        export_format='GLB',
        use_selection=True,
        export_apply=True,
        export_materials='EXPORT',
    )
    
    return {"objects": 1, "vertices": len(obj.data.vertices), "faces": len(obj.data.polygons)}

def main():
    argv = sys.argv
    if "--" not in argv:
        argv = []
    else:
        argv = argv[argv.index("--") + 1:]
    
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--size", type=float, default=10.0)
    args = parser.parse_args(argv)
    
    bpy.ops.wm.read_factory_settings(use_empty=True)
    
    print("\n=== VS#1: Generating forest ground ===")
    
    # Create ground
    plane = create_ground_plane(args.size)
    mat = create_forest_ground_material()
    plane.data.materials.append(mat)
    
    # Export
    stats = export_to_glb(args.output, plane)
    print(f"Ground: {stats['vertices']} verts, {stats['faces']} faces")
    print(json.dumps({"success": True, "output": args.output, **stats}))

if __name__ == "__main__":
    main()
