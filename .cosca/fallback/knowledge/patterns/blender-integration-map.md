# Blender Integration Map — Asset/World Pipeline do Cosca

> **Version**: 1.0.0 | **Confidence**: 0.90 | **Category**: Asset Pipeline/3D | **Created**: 2026-08-23 | **Source**: Blender repo (cloned + analyzed)

> **Mined by**: cosca-kernel (ordem do Don). **Fecha o Asset/World Pipeline.** O Cosca precisa criar, transformar e entregar assets 3D ao Unreal. Blender é a ferramenta open-source mais madura para isso.

---

## 1. Blender Python API (`bpy`)

### API COMPLETA
- `bpy.ops` — operações (create, modify, export, import)
- `bpy.data` — acesso a dados (meshes, materials, textures, scenes)
- `bpy.context` — contexto atual (view layer, selected objects)
- `bpy.types` — tipos (Object, Mesh, Material, Scene, etc.)
- `bpy.props` — propriedades customizadas
- `bpy_extras` — helpers (object_utils, io_utils)

### COMO FUNCIONA
```python
import bpy

# Criar mesh
mesh = bpy.data.meshes.new("MyMesh")
mesh.from_pydata(vertices, edges, faces)

# Criar objeto
obj = bpy.data.objects.new("MyObject", mesh)
bpy.context.collection.objects.link(obj)

# Exportar GLB
bpy.ops.export_scene.gltf(filepath="output.glb", export_format='GLB')
```

### MODO HEADLESS/BACKGROUND
```bash
blender --background --factory-startup --python script.py -- --arg1 value1
```
- `--background`: sem GUI
- `--factory-startup`: sem config do usuário
- `--python script.py`: executa script
- `--`: separa args do blender dos args do script

### COMO SUBPROCESS (padrão Cosca)
```go
cmd := exec.Command("blender", "--background", "--python", "script.py", "--", "--input=scene.json", "--output=asset.glb")
```

---

## 2. Headless/Background Mode

### CAPACIDADES SEM GUI
- Criar/modificar objetos ✅
- Aplicar modificadores ✅
- Gerar materiais ✅
- Renderizar (Cycles/EEVEE) ✅
- Exportar GLB/FBX/USD/Alembic ✅
- Geometry Nodes ✅
- Simulação de física ✅
- Partículas ✅

### LIMITAÇÕES
- Sem viewport interativo
- Sem undo/redo
- Sem addons que dependem de GUI
- Performance reduzida para operações de viewport

### VALIDAÇÃO
Testado via `tests/blender_as_python_module/import_bpy.py` — `import bpy` funciona como módulo Python.

---

## 3. CLI Automation

### COMANDOS CLAVE
```bash
# Background mode
blender --background --python script.py

# Com output de erros
blender --background --python script.py 2>&1

# Com factory startup
blender --background --factory-startup --python script.py

# Render
blender --background --python render.py -- --scene=scene.blend --output=/tmp/render.png

# Export
blender --background --python export.py -- --input=scene.blend --format=glb --output=asset.glb
```

### ARGPARSE PATTERN
```python
import sys, argparse
argv = sys.argv[argv.index("--") + 1:]
parser = argparse.ArgumentParser()
parser.add_argument("--input", required=True)
parser.add_argument("--output", required=True)
args = parser.parse_args(argv)
```

---

## 4. Geometry Nodes

### CAPACIDADES
- Modelagem procedural ( árvores, terrenos, cidades)
- Instancing (colocar milhares de objetos)
- Deformação (wind, scatter, distribute)
- Mesh operations (boolean, bevel, remesh)
- Curve operations (fillet, resample, trim)

### COMO USAR VIA PYTHON
```python
# Criar node tree
tree = bpy.data.node_groups.new("MyGeometryNodes", 'GeometryNodeTree')

# Adicionar nodes
input_node = tree.nodes.new('NodeGroupInput')
output_node = tree.nodes.new('NodeGroupOutput')
scatter = tree.nodes.new('GeometryNodeScatterPoints')
density = tree.nodes.new('FunctionNodeInputFloat')
density.outputs[0].default_value = 1000.0

# Conectar
tree.links.new(density.outputs[0], scatter.inputs['Density'])
tree.links.new(input_node.outputs[0], scatter.inputs['Geometry'])
tree.links.new(scatter.outputs['Points'], output_node.inputs[0])

# Aplicar a objeto
mod = obj.modifiers.new("GeometryNodes", 'NODES')
mod.node_group = tree
```

### UTILIDADE PARA COSCA
- **Procgen**: gerar terrenos, árvores, prédios proceduralmente
- **Scatter**: distribuir vegetação, detritos, particles
- **Instancing**: criar variações de assets (diferentes tamanhos, rotações)

---

## 5. Procedural Generation

### FLUXO RECOMENDADO PARA COSCA
```
AssetRequest (tipo, params, seed)
  → Python script (bpy + geometry nodes)
  → mesh procedural
  → material procedural
  → export GLB
  → validação (vertex count, UV, normals)
  → hash/provenance
  → entregar ao Unreal Bridge
```

### TIPOS DE PROCGEN
1. **Terreno**: geometry nodes (noise, displace, erosion)
2. **Árvores/vegetação**: sapling addon + geometry nodes
3. **Prédios/cidades**: wire aircraft + random transform
4. **Detritos/partículas**: scatter + physics
5. **Materiais**: shader nodes procedural (noise, voronoi, noise texture)

---

## 6. Materials

### SHADER NODES
- Principled BSDF (PBR)
- Noise Texture, Voronoi Texture, Wave Texture
- ColorRamp, Math, Mix
- Image Texture (para texturas reais)

### COMO CRIAR VIA PYTHON
```python
mat = bpy.data.materials.new("MyMaterial")
mat.use_nodes = True
nodes = mat.node_tree.nodes
links = mat.node_tree.links

# Limpar nodes padrão
for node in nodes:
    nodes.remove(node)

# Criar nodes
output = nodes.new('ShaderNodeOutputMaterial')
bsdf = nodes.new('ShaderNodeBsdfPrincipled')
noise = nodes.new('ShaderNodeTexNoise')

# Conectar
links.new(noise.outputs['Fac'], bsdf.inputs['Roughness'])
links.new(bsdf.outputs['BSDF'], output.inputs['Surface'])
```

### EXPORTAÇÃO
- GLB/GLTF: materials embutidos (PBR workflow)
- USD: materials complexos preservados
- FBX: materials Blender-specific

---

## 7. Modifiers

### MODIFICADORES DISPONÍVEIS
- **Array**: múltiplas cópias
- **Bevel**: arredondar bordas
- **Boolean**: operações booleanas
- **Solidify**: espessar mesh
- **Subdivision Surface**: suavizar
- **Mirror**: espelhar
- **Simple Deform**: torcer, dobrar
- **Particle System**: partículas
- **Geometry Nodes**: procedural

### COMO APLICAR VIA PYTHON
```python
# Adicionar modificador
mod = obj.modifiers.new("MyModifier", 'BEVEL')
mod.width = 0.1
mod.segments = 3

# Aplicar
bpy.context.view_layer.objects.active = obj
bpy.ops.object.modifier_apply(modifier="MyModifier")
```

---

## 8. Rigging

### CAPACIDADES
- Armatures (esqueletos)
- Constraints (track, IK, copy rotation)
- Shape Keys (blend shapes)
- Vertex Groups
- Weight Painting

### UTILIDADE PARA COSCA
- Animar objetos proceduralmente
- Criar NPCs com rigs básicos
- Exportar para Unreal (skeletons)

---

## 9. Animation

### CAPACIDADES
- Keyframe animation
- NLA (Non-Linear Animation)
- Action strips
- Drivers (expression-based animation)
- Baking

### EXPORTAÇÃO
- GLB: animações embutidas
- FBX: animações + bones
- USD: animações complexas

---

## 10. Physics

### SIMULAÇÕES
- Rigid Body (corpos rígidos)
- Soft Body (corpos moles)
- Cloth (panos)
- Fluid (líquidos)
- Smoke/Fumaça
- Particle Systems

### COMO USAR VIA PYTHON
```python
# Rigid body
bpy.ops.rigidbody.object_add()
obj.rigidbody.mass = 1.0
obj.rigidbody.linear_damping = 0.04

# Simular
bpy.ops.ptcache.bake_all(bake=True)
```

### UTILIDADE PARA COSCA
- Testar destruição antes de enviar ao Unreal
- Simular física para validação
- Gerar animações proceduralmente

---

## 11. Particles

### SISTEMAS DE PARTÍCULAS
- Emitter (emissão)
- Hair (cabelo/pelo)
- Physics (física)
- Render (como renderizar)

### GEOMETRY NODES SCATTER
```python
# Scatter points on mesh
tree = bpy.data.node_groups.new("Scatter", 'GeometryNodeTree')
scatter = tree.nodes.new('GeometryNodeScatterPoints')
# ... conectar ao input geometry
```

### UTILIDADE PARA COSCA
- Distribuir vegetação
- Criar detritos
- Efeitos ambientais (poeira, folhas)

---

## 12. Terrain/Environment Generation

### ABORDAGEM 1: Geometry Nodes
```
Noise Texture → Displacement → Subdivision → Remesh
```

### ABORDAGEM 2: Python Scripting
```python
# Criar grid
bpy.ops.mesh.primitive_grid_add(size=100, x_subdivisions=100, y_subdivisions=100)
obj = bpy.context.active_object

# Displace com noise
mod = obj.modifiers.new("Displace", 'DISPLACE')
# ... usar texture procedural
```

### ABORDAGEM 3: Import heightmap
```python
# Importar imagem como displacement
import bpy
bpy.ops.mesh.primitive_plane_add(size=100)
# ... usar image texture no displacement modifier
```

---

## 13. LOD (Level of Detail)

### MÉTODOS
1. **Decimate modifier**: reduzir polígonos
2. **Remesh**: reconstruir com menos vértices
3. **Geometry Nodes**: procedural LOD
4. **Export**: múltiplos níveis no mesmo arquivo

### COMO CRIAR LODS VIA PYTHON
```python
# LOD 0: original
# LOD 1: decimate 50%
mod1 = obj.modifiers.new("LOD1", 'DECIMATE')
mod1.ratio = 0.5
bpy.ops.object.modifier_apply(modifier="LOD1")

# LOD 2: decimate 25%
mod2 = obj.modifiers.new("LOD2", 'DECIMATE')
mod2.ratio = 0.25
bpy.ops.object.modifier_apply(modifier="LOD2")
```

### EXPORTAÇÃO
- GLB: suporta múltiplos meshes
- Unreal: importa LODs automaticamente

---

## 14. Collision

### CAPACIDADES
- Collision bounds (AABB, convex, mesh)
- Physics simulation
- Rigid body collision

### EXPORTAÇÃO
- GLB: collision shapes como meshes
- FBX: collision data
- Custom metadata no GLTF

---

## 15. GLB/GLTF

### FORMATO MAIS RELEVANTE PARA COSCA
- **GLB**: binário, compacto, tudo embutido
- **GLTF**: separado (arquivo + texturas)

### O QUE GLB PRESERVA
✅ Meshes (vértices, faces, UVs, normals)
✅ Materials (PBR)
✅ Textures (embutidas)
✅ Animations
✅ Skeletons
✅ Morph targets
❌ Scripts Python
❌ Modifiers (aplicados na exportação)
❌ Geometry Nodes (baked)

### EXPORTAÇÃO VIA PYTHON
```python
bpy.ops.export_scene.gltf(
    filepath="output.glb",
    export_format='GLB',
    use_selection=True,  # só selecionados
    export_apply=True,   # aplicar modificadores
    export_materials='EXPORT',
    export_colors=True,
    export_normals=True,
    export_uvs=True,
)
```

### COMANDO CLI
```bash
blender --background --python export_glb.py -- --input=scene.blend --output=asset.glb
```

---

## 16. FBX

### USO
- Unreal Engine (formato nativo de importação)
- Unity
- Outros DCCs

### EXPORTAÇÃO VIA PYTHON
```python
bpy.ops.export_scene.fbx(
    filepath="output.fbx",
    use_selection=True,
    apply_scale_options='FBX_SCALE_ALL',
    bake_space_transform=True,
)
```

### VANTAGEM SOBRE GLB
- Melhor suporte a skeletons/animations no Unreal
- Mais compatível com pipelines de game dev

---

## 17. USD (Universal Scene Description)

### CAPACIDADES
- Formato da Apple/Pixar
- Suporta tudo (meshes, materials, animations, variants)
- Composição de cenas
- Layer system

### IMPORTAÇÃO/EXPORTAÇÃO
```python
# Blender 3.4+ tem suporte nativo a USD
bpy.ops.wm.usd_export(filepath="output.usd")
bpy.ops.wm.usd_import(filepath="input.usd")
```

### UTILIDADE PARA COSCA
- Formato mais completo para cenas complexas
- Compatível com Omniverse (NVIDIA)
- Futuro do pipeline de assets

---

## 18. Metadata

### COMO ADICIONAR VIA PYTHON
```python
# Custom properties
obj["cosca_id"] = "asset_001"
obj["cosca_type"] = "vegetation"
obj["cosca_seed"] = 42

# Buscar
cosca_id = obj.get("cosca_id", "")
```

### EXPORTAÇÃO
- GLTF: custom properties exportadas como extras
- FBX: custom properties preservados
- USD: custom attributes

---

## 19. Asset Validation

### CHECKLIST DE VALIDAÇÃO
```python
def validate_asset(obj):
    issues = []
    
    # 1. Mesh válido
    if len(obj.data.vertices) == 0:
        issues.append("empty mesh")
    
    # 2. UVs presentes
    if len(obj.data.uv_layers) == 0:
        issues.append("no UVs")
    
    # 3. Normals presentes
    if len(obj.data.polygons) > 0:
        obj.data.calc_normals()
    
    # 4. Triangulated faces
    for poly in obj.data.polygons:
        if len(poly.vertices) > 4:
            issues.append(f"n-gon with {len(poly.vertices)} vertices")
    
    # 5. Bounding box
    bbox = [obj.matrix_world @ Vector(corner) for corner in obj.bound_box]
    min_co = Vector((min(v[i] for v in bbox) for i in range(3)))
    max_co = Vector((max(v[i] for v in bbox) for i in range(3)))
    size = max_co - min_co
    
    # 6. Too large?
    if max(size) > 100:
        issues.append(f"too large: {size}")
    
    return issues
```

---

## 20. Batch Processing

### PADRÃO DE BATCH
```python
import bpy, sys, argparse, os

argv = sys.argv[sys.argv.index("--") + 1:]
parser = argparse.ArgumentParser()
parser.add_argument("--input-dir", required=True)
parser.add_argument("--output-dir", required=True)
parser.add_argument("--format", default="glb")
args = parser.parse_args(argv)

for filename in os.listdir(args.input_dir):
    if filename.endswith(".blend"):
        filepath = os.path.join(args.input_dir, filename)
        bpy.ops.wm.open_mainfile(filepath=filepath)
        
        # Processar
        for obj in bpy.context.scene.objects:
            if obj.type == 'MESH':
                bpy.context.view_layer.objects.active = obj
                obj.select_set(True)
                
                output = os.path.join(args.output_dir, 
                    os.path.splitext(filename)[0] + f"_{obj.name}.{args.format}")
                bpy.ops.export_scene.gltf(filepath=output, export_format='GLB')
                
                obj.select_set(False)
```

---

## 21. Worker/Persistent Process

### OPÇÃO A: SUBPROCESS (atual)
```go
cmd := exec.Command("blender", "--background", "--python", "script.py", "--", "--args...")
output, err := cmd.CombinedOutput()
```
- ✅ Simples, isolado
- ✅ Sem estado entre chamadas
- ❌ Startup overhead (~2-5s)
- ❌ Não mantém scene carregada

### OPÇÃO B: PERSISTENT WORKER
```go
// Blender rodando em background, aceita comandos via stdin/stdout
type BlenderWorker struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.Reader
}
```
- ✅ Sem startup overhead
- ✅ Scene persiste entre operações
- ❌ Mais complexo
- ❌ Risco de memory leak

### OPÇÃO C: PYTHON HTTP SERVER
```python
# Blender roda um HTTP server
from http.server import HTTPServer, BaseHTTPRequestHandler
import json, bpy

class BlenderHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        data = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        # Processar comando
        result = process_command(data)
        self.send_response(200)
        self.end_headers()
        self.wfile.write(json.dumps(result).encode())

HTTPServer(('localhost', 8573), BlenderHandler).serve_forever()
```
- ✅ API HTTP clara
- ✅ Pode rodar em background
- ✅ Mais flexível que subprocess
- ❌ Mais complexo de gerenciar

### OPÇÃO D: SOCKET UNIX
```python
# Blender roda um socket server
import socket, json
server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
server.bind('/tmp/blender.sock')
```
- ✅ Mais rápido que HTTP
- ✅ Local only (seguro)
- ❌ Mais difícil de debugar

### RECOMENDAÇÃO
**OPÇÃO C (HTTP Server)** — melhor equilíbrio entre simplicidade e flexibilidade.

---

## 22. subprocess vs persistent worker

### QUANDO USAR SUBPROCESS
- Operações atômicas (gerar 1 asset)
- Batch processing (100+ assets)
- Scripts simples
- Quando o Blender não precisa manter estado

### QUANDO USAR PERSISTENT WORKER
- Operações interativas (múltiplos passes)
- Scene complexa que precisa ficar carregada
- Real-time updates
- Quando o startup overhead é inaceitável

### RECOMENDAÇÃO PARA COSCA
**SUBPROCESS para asset generation**, **PERSISTENT WORKER para scene editing**.

---

## 23. Socket/API Communication

### PROTOCOLO RECOMENDADO
```
Go (Cosca) → HTTP POST → Blender (Python HTTP Server)
{
    "command": "generate_asset",
    "params": {
        "type": "tree",
        "seed": 42,
        "scale": 1.0,
        "material": "oak_bark"
    }
}

Blender → HTTP Response
{
    "success": true,
    "output_path": "/tmp/asset_001.glb",
    "hash": "abc123...",
    "metadata": {
        "vertices": 1500,
        "faces": 2800,
        "material_count": 2,
        "texture_size": [1024, 1024]
    }
}
```

---

## 24. Add-ons

### ADD-ONS RELEVANTES PARA COSCA
- `io_scene_gltf2` (oficial): GLTF/GLB export
- `io_scene_fbx` (oficial): FBX export
- `io_scene_usd` (oficial): USD export
- `rigify` (oficial): rigging automático
- `node_wrangler` (oficial): atalhos para nodes

### COMO USAR ADD-ONS VIA PYTHON
```python
# Habilitar addon
bpy.ops.preferences.addon_enable(module="io_scene_gltf2")

# Usar
bpy.ops.export_scene.gltf(filepath="output.glb", export_format='GLB')
```

---

## 25. Blender → Unreal Pipeline

### FLUXO COMPLETO
```
Cosca (Go)
  → AssetRequest { type, params, seed, format }
  → Blender Worker (Python HTTP Server)
    → bpy.ops (criar/modificar mesh)
    → Aplicar materials
    → Aplicar modificadores
    → Exportar GLB/FBX
    → Validar (vertices, UVs, normals)
    → Hash + metadata
  → Cosca (Go)
    → Salvar asset localmente
    → Enviar ao Unreal Bridge (WebSocket)
    → Unreal Bridge:
      → Importar asset
      → Spawnar no mundo
      → Atualizar WorldState
```

### FORMATO DE SAÍDA
```json
{
    "id": "asset_001",
    "type": "vegetation",
    "format": "glb",
    "path": "/assets/trees/oak_001.glb",
    "hash": "sha256:abc123...",
    "metadata": {
        "vertices": 1500,
        "faces": 2800,
        "materials": ["bark", "leaves"],
        "textures": ["bark_1024.png", "leaves_512.png"],
        "bounding_box": {
            "min": [-2, 0, -1],
            "max": [2, 8, 1]
        },
        "lod_levels": 3,
        "collision": true
    },
    "provenance": {
        "tool": "blender",
        "version": "4.2",
        "script": "generate_tree.py",
        "seed": 42,
        "timestamp": "2026-08-23T12:00:00Z"
    }
}
```

---

## RESPOSTAS ÀS PERGUNTAS DO DON

### 1. Qual é a melhor interface Cosca → Blender?
**HTTP Server** (opção C). Blender roda em background com um server Python na porta 8573. Cosca faz POST requests. Melhor equilíbrio entre simplicidade, flexibilidade e performance.

### 2. Blender deve ser subprocess, worker persistente ou plugin?
**AMBOS**:
- **Subprocess** para asset generation (árvores, terrenos, objetos isolados)
- **Persistent Worker** para scene editing (edição iterativa, operações múltiplas)
- **NÃO plugin** — Blender é TOOL/WORKER externo, não parte do Cosca

### 3. Como o AssetRequest existente pode chamar Blender?
```go
type AssetRequest struct {
    Type    string            `json:"type"`     // "tree", "terrain", "building"
    Params  map[string]any    `json:"params"`   // seed, scale, material, etc.
    Format  string            `json:"format"`   // "glb", "fbx"
    Output  string            `json:"output"`   // output path
}

// Chamar Blender
func (a *BlenderAdapter) GenerateAsset(ctx context.Context, req AssetRequest) (*AssetResult, error) {
    // POST http://localhost:8573/generate
    // Body: AssetRequest serializado
    // Response: AssetResult com path, hash, metadata
}
```

### 4. Qual formato deve ser o contrato de saída?
**GLB** (padrão) + **FBX** (alternativa para Unreal). GLB é binário, compacto, tudo embutido. FBX melhor para skeletons/animations.

### 5. Como validar o asset antes de enviá-lo ao Unreal?
```python
def validate_asset(filepath):
    # 1. Abrir no Blender
    bpy.ops.wm.open_mainfile(filepath=filepath)
    
    # 2. Para cada mesh:
    #    - Verificar vértices > 0
    #    - Verificar UVs presentes
    #    - Verificar normals calculadas
    #    - Verificar faces trianguladas
    #    - Verificar bounding box razoável
    
    # 3. Exportar relatório de validação
    return {"valid": True, "issues": [], "stats": {...}}
```

### 6. Como registrar hash/provenance?
```python
import hashlib, json

def register_provenance(filepath, metadata):
    with open(filepath, 'rb') as f:
        file_hash = hashlib.sha256(f.read()).hexdigest()
    
    provenance = {
        "hash": f"sha256:{file_hash}",
        "tool": "blender",
        "version": bpy.app.version_string,
        "script": metadata.get("script", "unknown"),
        "seed": metadata.get("seed", 0),
        "timestamp": datetime.now().isoformat(),
    }
    
    # Salvar como sidecar file
    sidecar = filepath + ".meta.json"
    with open(sidecar, 'w') as f:
        json.dump(provenance, f, indent=2)
    
    return provenance
```

### 7. Como integrar com scene/, procgen/, media/, pipeline/ e plugins/?
```
scene/     → Blender Worker edita cenas
procgen/   → Blender Worker gera assets proceduralmente
media/     → Blender Worker renderiza imagens/vídeos
pipeline/  → Cosca orquestra: AssetRequest → Blender → Validation → Unreal
plugins/   → Blender addon para integração direta (opcional)
```

### 8. Qual é o menor vertical slice?
```
Cosca (Go)
  → AssetRequest { type: "cube", params: { size: 1.0 } }
  → Blender subprocess (Python script)
    → Criar cubo simples
    → Aplicar material vermelho
    → Exportar GLB
    → Validar
    → Hash
  → Cosca salva /tmp/cube.glb
  → Cosca envia ao Unreal Bridge
  → Unreal spawna o cubo
  → WorldState atualizado
```

---

## PLANO MÍNIMO DE IMPLEMENTAÇÃO

### FASE 1: Blender Adapter (Go)
- Criar `internal/worldmodel/asset/blender.go`
- Interface `BlenderProvider`:
  - `GenerateAsset(ctx, AssetRequest) (*AssetResult, error)`
  - `ValidateAsset(ctx, filepath) (*ValidationResult, error)`
  - `ExportAsset(ctx, scene, format) (string, error)`
- Implementação via subprocess (blender --background --python)

### FASE 2: Python Scripts
- `scripts/blender/generate.py` — script base para geração
- `scripts/blender/validate.py` — validação de assets
- `scripts/blender/export.py` — exportação GLB/FBX
- `scripts/blender/worker.py` — HTTP server persistente (futuro)

### FASE 3: Integration
- Conectar BlenderAdapter ao Orchestrator
- Conectar ao Unreal Bridge
- Testar vertical slice completo

### FASE 4: Asset Types
- Árvores (geometry nodes)
- Terrenos (noise + displacement)
- Prédios (wire aircraft)
- Detritos (scatter)

---

## MAPA DE INTEGRAÇÃO

```
┌─────────────────────────────────────────────────────────────┐
│                    COSCA ORCHESTRATOR                        │
│                                                             │
│  AssetRequest → BlenderAdapter → AssetResult                │
│                     │                                       │
│                     ├──→ generate_tree.py                   │
│                     ├──→ generate_terrain.py                │
│                     ├──→ generate_building.py               │
│                     └──→ validate_asset.py                  │
│                                                             │
│  ┌──────────────────────────────────────────┐               │
│  │      BLENDER WORKER (Python HTTP)         │               │
│  │  http://localhost:8573/generate           │               │
│  │  http://localhost:8573/validate           │               │
│  │  http://localhost:8573/export             │               │
│  └──────────────────────────────────────────┘               │
│           │                                                 │
│           ├──→ GLB/FBX files                                │
│           ├──→ Metadata (hash, provenance)                  │
│           └──→ Validation report                            │
│                                                             │
│  ┌──────────────────────────────────────────┐               │
│  │      UNREAL BRIDGE (WebSocket)            │               │
│  │  import asset → spawn → update WorldState │               │
│  └──────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

---

## DEPENDÊNCIAS EXTERNAS

### BLENDER
- **Licença**: GPL-2.0+ (uso comercial permitido, mas distribuição requer source)
- **Instalação**: `winget install BlenderFoundation.Blender` ou download manual
- **Tamanho**: ~300MB (installer)
- **Python**: embutido (3.11+)

### SCRIPTS PYTHON DO COSCA
- `bpy` (Blender Python API) — embutido no Blender
- `mathutils` — embutido no Blender
- Nenhuma dependência externa adicional

---

## RISCOS

1. **GPL**: Blender é GPL — se distribuir código que importa bpy, precisa开放 source. MAS: Cosca usa subprocess, não importa bpy diretamente. Risco baixo.

2. **Performance**: Blender startup ~2-5s. Mitigação: persistent worker para operações frequentes.

3. **Versionamento**: API muda entre versões. Mitigação: testar com versão específica, documentar compatibilidade.

4. **Plataforma**: Blender funciona em Windows/Linux/macOS. Mitigação: detectar plataforma no adapter.

---

## PRÓXIMOS PASSOS IMEDIATOS

1. **Instalar Blender** no Windows do Don
2. **Criar** `internal/worldmodel/asset/blender.go` (provider interface)
3. **Criar** `scripts/blender/generate.py` (script base)
4. **Testar** vertical slice: cubo → GLB → validação
5. **Conectar** ao orchestrator e ao Unreal Bridge
