#!/usr/bin/env python3
"""
Cosca VS#1 — Rock Generator

Generates a procedural rock with:
- Organic shape (deformed icosphere)
- Triplanar-friendly UV
- Bark-like material for testing

Usage:
  blender --background --factory-startup --python generate_rock.py -- --output /tmp/rock.glb
"""

import bpy
import bmesh
import sys
import json
import random
from mathutils import Vector

def create_rock(seed=42, size=1.0):
    """Create an organic rock shape."""
    random.seed(seed)
    
    # Start with icosphere
    bpy.ops.mesh.primitive_ico_sphere_add(
        subdivisions=3,
        radius=size,
        location=(0, 0, size * 0.3)
    )
    rock = bpy.context.active_object
    rock.name = "Rock"
    
    # Deform for organic look
    mesh = rock.data
    bm = bmesh.new()
    bm.from_mesh(mesh)
    
    for vert in bm.verts:
        # Random displacement
        noise = Vector((
            random.uniform(-0.3, 0.3),
            random.uniform(-0.3, 0.3),
            random.uniform(-0.2, 0.4),
        )) * size * 0.2
        vert.co += noise
        
        # Flatten bottom
        if vert.co.z < 0:
            vert.co.z *= 0.3
    
    bm.to_mesh(mesh)
    bm.free()
    
    # Recalculate normals
    bpy.ops.object.mode_set(mode='EDIT')
    bpy.ops.mesh.normals_make_consistent(inside=False)
    bpy.ops.object.mode_set(mode='OBJECT')
    
    # Material
    mat = bpy.data.materials.new("RockMaterial")
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs['Base Color'].default_value = (0.25, 0.22, 0.18, 1)
    bsdf.inputs['Roughness'].default_value = 0.9
    rock.data.materials.append(mat)
    
    return rock

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
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--size", type=float, default=1.0)
    args = parser.parse_args(argv)
    
    bpy.ops.wm.read_factory_settings(use_empty=True)
    
    print("\n=== VS#1: Generating rock ===")
    
    rock = create_rock(args.seed, args.size)
    stats = export_to_glb(args.output, rock)
    print(f"Rock: {stats['vertices']} verts, {stats['faces']} faces")
    print(json.dumps({"success": True, "output": args.output, **stats}))

if __name__ == "__main__":
    main()
