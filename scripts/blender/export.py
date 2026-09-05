#!/usr/bin/env python3
"""
Blender Asset Exporter for Cosca Living World.

Usage:
  blender --background --factory-startup --python export.py -- \
    --input /tmp/scene.blend --output /tmp/asset.glb --format glb

Exports the current Blender scene to the specified format.
"""

import bpy
import sys
import argparse
import json
import os


def main():
    # Parse arguments after "--"
    argv = sys.argv
    if "--" not in argv:
        argv = []
    else:
        argv = argv[argv.index("--") + 1:]
    
    parser = argparse.ArgumentParser(description="Export Blender asset")
    parser.add_argument("--input", required=True, help="Path to .blend file")
    parser.add_argument("--output", required=True, help="Output path")
    parser.add_argument("--format", default="glb", choices=["glb", "fbx", "usd"])
    parser.add_argument("--selection", action="store_true", help="Export selected objects only")
    
    args = parser.parse_args(argv)
    
    # Open blend file
    if args.input.endswith('.blend'):
        bpy.ops.wm.open_mainfile(filepath=args.input)
    else:
        print(json.dumps({'success': False, 'error': f'Unsupported input format: {args.input}'}))
        return
    
    # Export
    try:
        if args.format == "glb":
            bpy.ops.export_scene.gltf(
                filepath=args.output,
                export_format='GLB',
                use_selection=args.selection,
                export_apply=True,
                export_materials='EXPORT',
                export_colors=True,
                export_normals=True,
                export_uvs=True,
            )
        elif args.format == "fbx":
            bpy.ops.export_scene.fbx(
                filepath=args.output,
                use_selection=args.selection,
                apply_scale_options='FBX_SCALE_ALL',
                bake_space_transform=True,
            )
        elif args.format == "usd":
            bpy.ops.wm.usd_export(filepath=args.output)
        
        # Count exported objects
        object_count = len(bpy.context.selected_objects) if args.selection else len(bpy.context.scene.objects)
        
        result = {
            'success': True,
            'input': args.input,
            'output': args.output,
            'format': args.format,
            'object_count': object_count,
        }
        
    except Exception as e:
        result = {
            'success': False,
            'error': str(e),
        }
    
    print(json.dumps(result))


if __name__ == "__main__":
    main()
