#!/usr/bin/env python3
"""
Blender Asset Generator for Cosca Living World.

Usage:
  blender --background --factory-startup --python generate.py -- \
    --type tree --output /tmp/asset.glb --format glb --seed 42

This script generates procedural assets based on the requested type.
"""

import bpy
import sys
import argparse
import json
import os
import random
import hashlib
from datetime import datetime


def clear_scene():
    """Remove all objects from the scene."""
    bpy.ops.wm.read_factory_settings(use_empty=True)


def create_tree(params):
    """Create a procedural tree using geometry nodes."""
    seed = params.get('seed', 42)
    random.seed(seed)
    
    scale = params.get('scale', 1.0)
    trunk_height = params.get('trunk_height', 2.0) * scale
    canopy_radius = params.get('canopy_radius', 1.5) * scale
    
    # Create trunk (cylinder)
    bpy.ops.mesh.primitive_cylinder_add(
        radius=0.1 * scale,
        depth=trunk_height,
        location=(0, 0, trunk_height / 2)
    )
    trunk = bpy.context.active_object
    trunk.name = "Trunk"
    
    # Create canopy (sphere)
    bpy.ops.mesh.primitive_uv_sphere_add(
        radius=canopy_radius,
        location=(0, 0, trunk_height + canopy_radius * 0.5)
    )
    canopy = bpy.context.active_object
    canopy.name = "Canopy"
    
    # Apply simple material to trunk
    mat_trunk = bpy.data.materials.new(name="TrunkMaterial")
    mat_trunk.use_nodes = True
    bsdf = mat_trunk.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (0.35, 0.2, 0.1, 1)  # Brown
    bsdf.inputs['Roughness'].default_value = 0.8
    trunk.data.materials.append(mat_trunk)
    
    # Apply simple material to canopy
    mat_canopy = bpy.data.materials.new(name="CanopyMaterial")
    mat_canopy.use_nodes = True
    bsdf = mat_canopy.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (0.1, 0.4, 0.1, 1)  # Green
    bsdf.inputs['Roughness'].default_value = 0.6
    canopy.data.materials.append(mat_canopy)
    
    return [trunk, canopy]


def create_cube(params):
    """Create a simple cube."""
    size = params.get('size', 1.0)
    
    bpy.ops.mesh.primitive_cube_add(size=size, location=(0, 0, size / 2))
    cube = bpy.context.active_object
    cube.name = "Cube"
    
    # Apply material
    mat = bpy.data.materials.new(name="CubeMaterial")
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (0.8, 0.2, 0.2, 1)  # Red
    bsdf.inputs['Roughness'].default_value = 0.5
    cube.data.materials.append(mat)
    
    return [cube]


def create_terrain(params):
    """Create a procedural terrain using displacement."""
    seed = params.get('seed', 42)
    random.seed(seed)
    
    size = params.get('size', 10.0)
    subdivisions = params.get('subdivisions', 64)
    height = params.get('height', 2.0)
    
    # Create grid
    bpy.ops.mesh.primitive_grid_add(size=size, x_subdivisions=subdivisions, y_subdivisions=subdivisions)
    terrain = bpy.context.active_object
    terrain.name = "Terrain"
    
    # Add displacement modifier
    mod = terrain.modifiers.new("Displace", 'DISPLACE')
    
    # Create texture
    tex = bpy.data.textures.new("TerrainTexture", 'CLOUDS')
    tex.noise_scale = size / 4
    mod.texture = tex
    mod.strength = height
    
    # Apply material
    mat = bpy.data.materials.new(name="TerrainMaterial")
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (0.3, 0.5, 0.2, 1)  # Green-brown
    bsdf.inputs['Roughness'].default_value = 0.9
    terrain.data.materials.append(mat)
    
    return [terrain]


def create_building(params):
    """Create a simple building."""
    width = params.get('width', 4.0)
    depth = params.get('depth', 4.0)
    height = params.get('height', 6.0)
    floors = params.get('floors', 2)
    
    # Create main structure
    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, height / 2))
    building = bpy.context.active_object
    building.name = "Building"
    building.scale = (width / 2, depth / 2, height / 2)
    bpy.ops.object.transform_apply(scale=True)
    
    # Apply material
    mat = bpy.data.materials.new(name="BuildingMaterial")
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (0.6, 0.6, 0.6, 1)  # Gray
    bsdf.inputs['Roughness'].default_value = 0.7
    building.data.materials.append(mat)
    
    return [building]


# Generator mapping
GENERATORS = {
    'tree': create_tree,
    'cube': create_cube,
    'terrain': create_terrain,
    'building': create_building,
}


def main():
    # Parse arguments after "--"
    argv = sys.argv
    if "--" not in argv:
        argv = []
    else:
        argv = argv[argv.index("--") + 1:]
    
    parser = argparse.ArgumentParser(description="Generate Blender asset")
    parser.add_argument("--type", required=True, choices=GENERATORS.keys())
    parser.add_argument("--output", required=True)
    parser.add_argument("--format", default="glb", choices=["glb", "fbx", "usd"])
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--scale", type=float, default=1.0)
    parser.add_argument("--validate", action="store_true")
    
    # Additional params
    parser.add_argument("--trunk-height", type=float, default=2.0)
    parser.add_argument("--canopy-radius", type=float, default=1.5)
    parser.add_argument("--size", type=float, default=1.0)
    parser.add_argument("--height", type=float, default=6.0)
    parser.add_argument("--width", type=float, default=4.0)
    parser.add_argument("--depth", type=float, default=4.0)
    parser.add_argument("--floors", type=int, default=2)
    parser.add_argument("--subdivisions", type=int, default=64)
    
    args = parser.parse_args(argv)
    
    # Build params dict
    params = {
        'seed': args.seed,
        'scale': args.scale,
        'trunk_height': args.trunk_height,
        'canopy_radius': args.canopy_radius,
        'size': args.size,
        'height': args.height,
        'width': args.width,
        'depth': args.depth,
        'floors': args.floors,
        'subdivisions': args.subdivisions,
    }
    
    # Clear scene
    clear_scene()
    
    # Generate asset
    generator = GENERATORS[args.type]
    objects = generator(params)
    
    # Export
    if args.format == "glb":
        bpy.ops.export_scene.gltf(
            filepath=args.output,
            export_format='GLB',
            use_selection=False,
            export_apply=True,
            export_materials='EXPORT',
        )
    elif args.format == "fbx":
        bpy.ops.export_scene.fbx(
            filepath=args.output,
            use_selection=False,
            apply_scale_options='FBX_SCALE_ALL',
        )
    elif args.format == "usd":
        bpy.ops.wm.usd_export(filepath=args.output)
    
    # Output metadata
    metadata = {
        'success': True,
        'type': args.type,
        'output': args.output,
        'format': args.format,
        'objects': [obj.name for obj in objects],
    }
    
    print(json.dumps(metadata))


if __name__ == "__main__":
    main()
