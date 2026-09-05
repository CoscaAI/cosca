# Camada 6: 🗺️ Spatial AI — O Agente Entende o Espaço

> **Version**: 1.0.0 | **Confidence**: 0.83 | **Category**: Spatial/3D Understanding | **Created**: 2026-08-23 | **Source**: GitHub API (stars/license) + conhecimento + template do professor

> **Mined by**: cosca-kernel (ordem do Don). **Gap #2 do mining-map-world-vivo.md.** O agente não só enxerga o mundo (Vision), mas precisa ENTENDER o espaço — onde está, como navegar, o que é vizinho do quê, como reconstruir 3D. Template: REPO→PAPER→MODEL→LICENSE→DEPS→BENCHMARK→INTEGRATION→PLUGIN→UTILITY.

## Purpose

Minar a camada **Spatial AI** do Mundo Vivo. O agente precisa: (1) mapear o mundo em 3D, (2) navegar nele, (3) reconstruir geometria, (4) entender relações espaciais. Cada projeto abaixo é uma **capacidade espacial** que o Cosca pode dar ao agente.

---

## 1. ORB-SLAM3

### REPO
`UZ-SLAMLab/ORB_SLAM3` — ★9k — GPL-3.0 — C++

### PAPER/MÉTODO
ORB-SLAM3 é o **SLAM visual clássico** — simultaneamente localiza o agente no mundo e constrói um mapa. Usa features ORB (fast keypoint detection + binary descriptor) + bundle adjustment + loop closure. Suporta: monocular, stereo, IMU, RGB-D. Multi-map (divide o mundo em mapas locais).

### MODELO
Não é neural — é geometria clássica (features + otimização). Binário compilado (C++).

### LICENSE
GPL-3.0 ⚠️ (requer open-source do código que usa; para o Cosca interno é OK; para distribuição, precisa de licença)

### DEPENDÊNCIAS
OpenCV, Eigen, DBoW2, g2o. Compilação C++ complexa. CPU (sem GPU necessária).

### BENCHMARK
State-of-the-art em SLAM clássico (TUM, EuRoC, KITTI). Muito robusto em ambientes reais.

### INTEGRAÇÃO NO COSCA
ORB-SLAM3 é o **GPS do agente** — ele sabe onde está no mundo em tempo real. O mapa 3D gerado (point cloud) pode ser consumido pelo Cosca para navegação e planejamento. Funciona sem GPU (CPU puro), o que é bom para rodar em hardware leve.

### POSSÍVEL PLUGIN
`cosca-spatial-slam` — plugin que recebe frames de câmera → retorna pose (posição+orientação) + mapa de pontos 3D. Input: frame + IMU (opcional). Output: pose 6DoF + point cloud.

### UTILIDADE NO COSCA
**Alta.** Sem SLAM, o agente não sabe onde está. ORB-SLAM3 é o **sistema de navegação** — ele localiza o agente e mapeia o mundo ao redor. Funciona como fallback quando os neurais não estão disponíveis.

---

## 2. NICE-SLAM

### REPO
`cvg/nice-slam` — ★1.6k — Apache-2.0 — Python

### PAPER/MÉTODO
NICE-SLAM usa **grid features** (não MLP puro) para representar o mundo como um mapa 3D neural hierárquico. Mais rápido e escalável que NeRF-SLAM. Arquitetura: hierarchical scene representation (coarse→fine) + pose estimation + bundle adjustment neural. Otimização em tempo real.

### MODELO
Grid features (tensor 3D) + pose optimizer. Pesos open-source (PyTorch).

### LICENSE
Apache-2.0 ✅ (compatível com Cosca)

### DEPENDÊNCIAS
PyTorch, numpy, OpenCV. GPU moderada (~4GB).

### BENCHMARK
Competitivo com ORB-SLAM3 em ambientes indoor, melhor em texturas fracas. Mais robusto que NeRF-SLAM.

### INTEGRAÇÃO NO COSCA
NICE-SLAM é o **SLAM neural** — mais robusto que ORB-SLAM3 em ambientes difíceis (pouca textura, iluminação ruim). O mapa gerado é uma representação neural do mundo que pode ser consultada ("o que tem nesta região?").

### POSSÍVEL PLUGIN
`cosca-spatial-nice-slam` — plugin neural que recebe frames → retorna mapa 3D neural + pose. Mais pesado que ORB-SLAM3 mas mais robusto.

### UTILIDADE NO COSCA
**Média-Alta.** NICE-SLAM é o **SLAM neural** — quando ORB-SLAM3 falha (pouca textura), NICE-SLAM funciona. Pode ser usado em paralelo como fallback.

---

## 3. Instant-NGP

### REPO
`NVlabs/instant-ngp` — ★17.5k — Custom (NVIDIA) — CUDA

### PAPER/MÉTODO
Instant Neural Graphics Primitives — treina NeRFs em **segundos** (não horas). Usa **hash encoding** (multiresolution hash table) para acelerar o treinamento neural. Resultado: reconstrução 3D de alta qualidade a partir de fotos/vídeo.

### MODELO
Hash encoding + tiny MLP. Pesos open-source (CUDA). Precisa de GPU NVIDIA.

### LICENSE
Custom (NVIDIA) ⚠️ — permite uso pessoal/research; comercial requer licença NVIDIA. Verificar termos.

### DEPENDÊNCIAS
CUDA, tiny-cuda-nn, pytorch. GPU NVIDIA obrigatória (~8GB+).

### BENCHMARK
1000x mais rápido que NeRF original. Qualidade state-of-the-art em novel view synthesis.

### INTEGRAÇÃO NO COSCA
Instant-NGP é o **fotogrametrista rápido** — dado fotos/vídeo de um lugar, gera uma representação 3D completa em segundos. O agente pode "escanear" o mundo e ter uma cópia 3D dele. Liga à memória visual (DINOv2) e ao mapa (SLAM).

### POSSÍVEL PLUGIN
`cosca-spatial-instant-ngp` — plugin que recebe fotos/vídeo → retorna mesh 3D + textura. Input: conjunto de fotos. Output: mesh OBJ/FBX + texturas.

### UTILIDADE NO COSCA
**Alta.** Instant-NGP é o **scanner 3D do agente** — ele pode "fotografar" o mundo e gerar uma cópia 3D navegável. Essencial para o agente "construir" o mundo a partir de observação.

---

## 4. NeRF (original)

### REPO
`bmild/nerf` — ★10.9k — MIT — Python/Jupyter

### PAPER/MÉTODO
Neural Radiance Fields —Representation neural contínua de cena 3D (MLP que mapeia posição+direção→cor+densidade). Dado fotos, otimiza o MLP para renderizar a cena de qualquer viewpoint. É a **base** de todos os NeRFs posteriores.

### MODELO
MLP simples (~5MB). Pesos open-source.

### LICENSE
MIT ✅ (tudo liberado)

### DEPENDÊNCIAS
PyTorch, numpy. GPU moderada (~4GB).

### BENCHMARK
Referência acadêmica. Qualidade alta mas treinamento lento (~horas).

### INTEGRAÇÃO NO COSCA
NeRF é a **base conceitual** — entende-lo é essencial para o Cosca trabalhar com representações neurais 3D. Para uso prático, Instant-NGP é melhor (mais rápido). Mas NeRF original é MIT e leve.

### POSSÍVEL PLUGIN
`cosca-spatial-nerf` — plugin leve para reconstrução 3D quando Instant-NGP não está disponível (GPU fraca).

### UTILIDADE NO COSCA
**Média.** NeRF é mais acadêmico que prático para o Cosca, mas é a base de Instant-NGP e NICE-SLAM. Conhecer é importante; usar Instant-NGP para produção.

---

## 5. Meshroom (AliceVision)

### REPO
`alicevision/Meshroom` — ★12.9k — Custom (AliceVision) — Python/C++

### PAPER/MÉTODO
Meshroom é um **pipeline de fotogrametria completo** com UI gráfica. AliceVision é o backend (SfM + MVS + Meshing + Texturing). Pipeline:照片 → Feature Extraction → Matching → Sparse Reconstruction (SfM) → Dense Reconstruction (MVS) → Mesh → Textured Model.

### MODELO
Não é neural — é geometria clássica (SfM+MVS). Binário compilado + Python.

### LICENSE
Custom (AliceVision/Meshroom) ⚠️ — open-source mas com restrições de uso comercial. Verificar.

### DEPENDÊNCIAS
AliceVision (C++), Qt (UI), Python. Pesado (~2GB disco). CPU (GPU opcional para MVS).

### BENCHMARK
State-of-the-art em fotogrametria clássica. Muito usado em VFX e game dev.

### INTEGRAÇÃO NO COSCA
Meshroom é o **pipeline de reconstrução completo** — se o Don quiser gerar assets 3D reais a partir de fotos, Meshroom é a escolha madura. Pode rodar como subprocesso (CLI) do Cosca.

### POSSÍVEL PLUGIN
`cosca-spatial-meshroom` — plugin que recebe fotos → retorna mesh texturizado via pipeline AliceVision.

### UTILIDADE NO COSCA
**Média-Alta.** Meshroom é mais maduro que Instant-NGP para assets de game (mesh limpo + textura). Útil para gerar **assets reais** do mundo.

---

## 6. MASt3R-SLAM

### REPO
`rmurai0610/MASt3R-SLAM` — ★3.2k — Python

### PAPER/MÉTODO
MASt3R-SLAM combina **DUSt3R** ( Dense Unified Stereo 3D Reconstruction) com SLAM. Usa transformers para correspondência stereo e gera mapas 3D dense diretamente de frames. Mais moderno que ORB-SLAM3 e NICE-SLAM.

### MODELO
Transformer-based (DUSt3R). Pesos open-source.

### LICENSE
Não especificada (verificar)

### DEPENDÊNCIAS
PyTorch, transformers. GPU (~6GB).

### BENCHMARK
State-of-the-art em 3D reconstruction dense. Mais rápido que NICE-SLAM.

### INTEGRAÇÃO NO COSCA
MASt3R-SLAM é o **SLAM moderno** — combina a robustez neural com a velocidade de SLAM. Bom candidato a "SLAM principal" do agente.

### POSSÍVEL PLUGIN
`cosca-spatial-master3r-slam` — plugin neural que recebe frames → retorna mapa 3D dense + pose.

### UTILIDADE NO COSCA
**Alta.** MASt3R-SLAM é o estado da arte — se for estável, é o melhor SLAM para o agente.

---

## 7. Spatial Reasoning (VLMs espaciais)

### REPO
- `InternRobotics/G2VLM` — ★350 — Apache-2.0 — Python (vision-language model para raciocínio espacial)
- `PzySeere/MetaSpatial` — ★320 — Apache-2.0 — Python (raciocínio espacial meta-aprendido)
- `zhangquanchen/3DThinker` — ★246 — Apache-2.0 — Python (pensamento 3D)

### PAPER/MÉTODO
Modelos vision-language que entendem **relações espaciais** ("a bola está à esquerda da caixa", "atrás da parede"). Usam VLMs treinados com dados espaciais para raciocinar sobre posição, orientação, distância.

### MODELO
VLMs (vision-language models). Pesos open-source.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
PyTorch, transformers. GPU moderada (~4GB).

### BENCHMARK
G2VLM: state-of-the-art em spatial VQA. MetaSpatial: good generalization.

### INTEGRAÇÃO NO COSCA
O agente não só vê (Vision) e mapeia (SLAM), mas **entende relações espaciais** ("o que está perto do que", "o que está atrás de quê"). Essencial para planejamento e navegação.

### POSSÍVEL PLUGIN
`cosca-spatial-reasoning` — plugin que recebe imagem + query espacial → resposta. Input: frame + "o que está à esquerda da mesa?". Output: objeto + relação.

### UTILIDADE NO COSCA
**Média-Alta.** Raciocínio espacial é a **inteligência** por trás da navegação — o agente não só sabe onde está, mas entende o mundo ao redor.

---

## Síntese da Camada Spatial AI

| # | Projeto | ★ | License | Capacidade no Cosca |
|---|---------|---|---------|---------------------|
| 1 | ORB-SLAM3 | 9k | GPL-3.0 | Localização+mapeamento clássico (GPS do agente) |
| 2 | NICE-SLAM | 1.6k | Apache-2.0 | SLAM neural (fallback robusto) |
| 3 | Instant-NGP | 17.5k | Custom | Scanner 3D rápido (fotos→mesh) |
| 4 | NeRF | 10.9k | MIT | Base conceitual (reconstrução neural) |
| 5 | Meshroom | 12.9k | Custom | Pipeline fotogrametria completo (fotos→asset) |
| 6 | MASt3R-SLAM | 3.2k | ? | SLAM moderno (estado da arte) |
| 7 | Spatial Reasoning | ~900 | Apache-2.0 | Raciocínio espacial (entende relações) |

### O pipeline espacial do agente

```
Câmera do agente
    │
    ├─→ ORB-SLAM3 / MASt3R-SLAM → "onde estou?" (pose + mapa)
    │
    ├─→ Instant-NGP / Meshroom → "como é o mundo?" (mesh 3D)
    │
    └─→ Spatial Reasoning VLM → "o que está perto/longe/atras?" (relações)
```

### Prioridade de instalação (quando o NVMe chegar)
1. **ORB-SLAM3** (GPS do agente — localização em tempo real)
2. **MASt3R-SLAM** (se estável, substitui ORB como principal)
3. **Instant-NGP** (scanner 3D — fotos→mesh)
4. **Spatial Reasoning VLM** (entende relações)
5. **NICE-SLAM** (fallback neural)
6. **Meshroom** (pipeline completo para assets)
7. **NeRF** (base conceitual)

## Related Patterns

- [`vision-layer-patterns.md`](vision-layer-patterns.md) — Camada 5: o agente enxerga (feeds a Spatial AI)
- [`mining-map-world-vivo.md`](mining-map-world-vivo.md) — Gap #2 (Spatial AI) é esta camada
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — World Partition (particionamento espacial no UE)
- [`generative-media-patterns.md`](generative-media-patterns.md) — Gaussian Splatting (reconstrução 3D)
