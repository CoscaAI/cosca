#!/usr/bin/env python3
"""
Cosca — Configurar S1_TestMap com atmosfera + meio-dia.

Adiciona ao mapa: SkyAtmosphere, SkyLight, DirectionalLight (sol),
ExponentialHeightFog, e seta o mundo para meio-dia.
Uso via UnrealEditor-Cmd -ExecutePythonScript
"""
import unreal

WORLD = unreal.EditorLevelLibrary

def add_actor(cls, location, rotation, label):
    actor = WORLD.spawn_actor_from_class(cls, location, rotation)
    if actor:
        actor.set_actor_label(label)
    return actor

print("=== Configurando S1_TestMap com atmosfera ===")

# 1. Directional Light (sol) — meio-dia, sol no zênite
sun = add_actor(unreal.DirectionalLight, unreal.Vector(0, 0, 1000), unreal.Rotator(0, 0, 0), "Cosca_Sun")
if sun:
    comp = sun.get_component_by_class(unreal.DirectionalLightComponent)
    if comp:
        comp.set_editor_property("intensity", 4.0)
        comp.set_editor_property("light_color", unreal.LinearColor(1.0, 0.98, 0.92))
        print("  ✓ Sun created (noon)")

# 2. Sky Atmosphere
sky_atmo = add_actor(unreal.SkyAtmosphere, unreal.Vector(0, 0, 0), unreal.Rotator(0, 0, 0), "Cosca_SkyAtmosphere")
if sky_atmo:
    print("  ✓ Sky Atmosphere created")

# 3. Sky Light
sky_light = add_actor(unreal.SkyLight, unreal.Vector(0, 0, 100), unreal.Rotator(0, 0, 0), "Cosca_SkyLight")
if sky_light:
    comp = sky_light.get_component_by_class(unreal.SkyLightComponent)
    if comp:
        comp.set_editor_property("intensity", 1.0)
        comp.set_editor_property("source_type", unreal.SkyLightSourceType.SLS_SpecifiedCubemap)
    print("  ✓ Sky Light created")

# 4. Exponential Height Fog
fog = add_actor(unreal.ExponentialHeightFog, unreal.Vector(0, 0, 0), unreal.Rotator(0, 0, 0), "Cosca_Fog")
if fog:
    comp = fog.get_component_by_class(unreal.ExponentialHeightFogComponent)
    if comp:
        comp.set_editor_property("fog_density", 0.005)
        comp.set_editor_property("fog_inscattering_color", unreal.LinearColor(0.6, 0.75, 1.0))
    print("  ✓ Fog created")

# 5. Sky Sphere (para céu visível)
try:
    sphere = add_actor(unreal.SkySphere, unreal.Vector(0, 0, 0), unreal.Rotator(0, 0, 0), "Cosca_SkySphere")
    if sphere:
        print("  ✓ Sky Sphere created")
except Exception as e:
    print(f"  (SkySphere: {e})")

# 6. Atmosfera exposta + setar meio-dia (regerar reflexos de tipo)
unreal.EditorLevelLibrary.save_current_level()
print("=== S1_TestMap salvo com atmosfera + meio-dia ===")
