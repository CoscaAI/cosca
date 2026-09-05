# Camada 5: 👁️ Vision — O Agente Enxerga o Mundo

> **Version**: 1.0.0 | **Confidence**: 0.85 | **Category**: Vision/Sensing | **Created**: 2026-08-23 | **Source**: GitHub API (stars/license) + conhecimento + template do professor

> **Mined by**: cosca-kernel (ordem do Don). **Gap #1 do mining-map-world-vivo.md.** O agente precisa ENXERGAR o mundo pra interagir — sem visão, ele é cego no mundo. Template: REPO→PAPER→MODEL→LICENSE→DEPS→BENCHMARK→INTEGRATION→PLUGIN→UTILITY.

## Purpose

Minar a camada **Vision** do Mundo Vivo. O agente precisa: (1) ver o mundo, (2) segmentar o que importa, (3) detectar objetos, (4) entender profundidade, (5) conectar visão à linguagem. Cada projeto abaixo é uma **capacidade visual** que o Cosca pode dar ao agente.

---

## 1. Segment Anything Model 2 (SAM2)

### REPO
`facebookresearch/sam2` — ★19.7k — Apache-2.0 — Jupyter Notebook/Python

### PAPER/MÉTODO
SAM2 (Segment Anything Model 2) estende SAM para vídeo/temporal. Usa **prompt engineering** (ponto/caixa/máscara) para segmentar qualquer objeto em imagem ou vídeo. Arquitetura: **image encoder** (ViT) + **prompt encoder** + **mask decoder**. Para vídeo, usa **memory attention** (transformer que attende a frames anteriores).

### MODELO
ViT-H/L/SAM-hiera (transformers pesados). SAM2 tiny (mais leve). pesos open-source no repo.

### LICENSE
Apache-2.0 ✅ (compatível com Cosca — uso comercial permitido)

### DEPENDÊNCIAS
PyTorch, transformers, OpenCV, numpy. Pesado (~2GB GPU para SAM2-large).

### BENCHMARK
State-of-the-art em segmentação zero-shot. Muito mais rápido que SAM1 para vídeos (memory attention). Benchmark: SA-V (video segmentation), COCO (image segmentation).

### INTEGRAÇÃO NO COSCA
O agente aponta para um ponto no mundo → SAM2 segmenta o objeto → o Cosca sabe "o que é o que" no mundo visual. Pode ser chamado via API Python (subprocesso do Cosca) ou como modelo ONNX via `internal/` (se o Don quiser embutir).

### POSSÍVEL PLUGIN
`cosca-vision-segment` — plugin que recebe uma imagem/frame do mundo e retorna máscaras de objetos. Input: frame (PNG/bytes). Output: lista de objetos segmentados (position, mask, label via GroundingDINO).

### UTILIDADE NO COSCA
**Alta.** O agente precisa saber "onde está o que" no mundo. SAM2 é o **olho que segmenta** — sem ele, o agente vê mas não entende o que vê.

---

## 2. CLIP (Contrastive Language-Image Pre-training)

### REPO
`openai/CLIP` — ★28k — MIT — Python

### PAPER/MÉTODO
CLIP treina um **image encoder** e um **text encoder** juntos (contrastive learning em 400M pares imagem-texto). Resultado: embeddings compartilhados — "gato" e imagem de gato ficam perto no espaço vetorial. Zero-shot classification: compara embedding da imagem com embeddings de texto.

### MODELO
ViT-B/32, ViT-B/16, ViT-L/14, ViT-H-14 (OpenCLIP). Pesos open-source.

### LICENSE
MIT ✅ (uso comercial, modificação, distribuição — tudo liberado)

### DEPENDÊNCIAS
PyTorch, torchvision. Leve (~300MB GPU).

### BENCHMARK
Zero-shot ImageNet accuracy: 76.2% (ViT-L/14) — sem treino específico. State-of-the-art em retrieval imagem-texto.

### INTEGRAÇÃO NO COSCA
O agente vê algo → CLIP conecta "o que vê" à "linguagem do Cosca". Ex: agente olha → CLIP diz "isso parece uma árvore". É a **ponte entre visão e linguagem** — o agente descreve o mundo em termos que o Cosca entende.

### POSSÍVEL PLUGIN
`cosca-vision-clip` — plugin que recebe imagem + candidatos de texto → retorna o mais provável. Input: frame + lista de labels. Output: label + score.

### UTILIDADE NO COSCA
**Alta.** CLIP é o **dicionário visual do agente** — conecta o que os olhos veem ao que o cérebro entende. Sem CLIP, o agente segmenta mas não sabe o nome das coisas.

---

## 3. GroundingDINO

### REPO
`IDEA-Research/GroundingDINO` — ★10.5k — Apache-2.0 — Python

### PAPER/MÉTODO
GroundingDINO faz **detecção grounded** — dado um texto ("a red car"), detecta e localiza o objeto na imagem com bounding box. Combina DINO (self-supervised detection) com grounding linguístico (text encoder). Arquitetura: **image backbone** + **text backbone** + **feature enhancer** + **grounding DINO decoder**.

### MODELO
GroundingDINO-T/Swint, GroundingDINO-B/SwinB. Pesos open-source.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
PyTorch, transformers, supervision. Moderado (~1GB GPU).

### BENCHMARK
State-of-the-art em zero-shot object detection (referring expression). COCO zero-shot: 52.5 AP.

### INTEGRAÇÃO NO COSCA
O agente olha → GroundingDINO detecta objetos por descrição textual. Ex: "encontre a porta" → bounding box da porta. É a **localização por linguagem** — o agente busca no mundo usando palavras.

### POSSÍVEL PLUGIN
`cosca-vision-detect` — plugin que recebe imagem + query textual → retorna bounding boxes + labels. Input: frame + "where is the X". Output: lista de objetos (bbox, label, score).

### UTILIDADE NO COSCA
**Alta.** GroundingDINO é o ** ponteiro do agente** — ele aponta pro mundo usando linguagem. Combinado com SAM2: "segmente a porta" → máscara perfeita da porta.

---

## 4. DINOv2

### REPO
`facebookresearch/dinov2` — ★13.2k — Apache-2.0 — Python

### PAPER/MÉTODO
DINOv2 treina ViTs com **self-supervised learning** (sem labels) em dados de larga escala (LVD-142M). Resultado: features visuais genéricas que servem para tudo — classificação, segmentação, correspondência, retrieval. É um **foundation model visual**.

### MODELO
ViT-S/14, ViT-B/14, ViT-L/14, ViT-g/14. Pesos open-source.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
PyTorch, torchvision. Leve (~200MB GPU para inference).

### BENCHMARK
State-of-the-art em transfer learning visual (ImageNet, ADE20K, COCO). Features mais ricas que DINOv1.

### INTEGRAÇÃO NO COSCA
DINOv2 é o **cérebro visual pré-treinado** do agente — extrai features ricas de qualquer imagem. Pode ser usado para: correspondência visual ("onde já vi isso?"), similaridade ("isso é parecido com aquilo"), clustering visual ("estes objetos são do mesmo tipo"). Liga à memória visual do agente.

### POSSÍVEL PLUGIN
`cosca-vision-features` — plugin que recebe imagem → retorna embedding visual (768/1024-dim). Input: frame. Output: feature vector + metadata.

### UTILIDADE NO COSCA
**Média-Alta.** DINOv2 é o **extrator de características visuais** — não é "o que é" (CLIP faz isso) mas "como é" (forma, textura, contexto). Útil para memória visual do agente.

---

## 5. YOLO (Ultralytics)

### REPO
`ultralytics/ultralytics` — ★60.8k — AGPL-3.0 — Python

### PAPER/MÉTODO
YOLO (You Only Look Once) é detecção de objetos **em tempo real**. YOLOv8/v11: arquitetura unified (backbone + neck + head), treinado em COCO. Velocidade > precisão (real-time). Suporta: detecção, segmentação, pose, classificação.

### MODELO
yolov8n/s/m/l/x, yolov11n/s/m/l/x. Pesos open-source. Exporta ONNX/TensorRT.

### LICENSE
AGPL-3.0 ⚠️ (uso comercial requer licença paga da Ultralytics; para o Cosca interno/personal, AGPL permite; para distribuição comercial, precisa de licença)

### DEPENDÊNCIAS
PyTorch, ultralytics. Leve (~100MB GPU para YOLOv8n).

### BENCHMARK
YOLOv8x: 53.9 AP COCO, 5ms/image (A100). YOLOv11x: 54.7 AP. Real-time é o ponto forte.

### INTEGRAÇÃO NO COSCA
YOLO é o **detector rápido do agente** — quando o agente precisa saber "o que tem aqui" em tempo real, YOLO é mais rápido que GroundingDINO. Útil para: monitoramento contínuo, alertas ("tem alguém atrás de mim"), contagem de objetos.

### POSSÍVEL PLUGIN
`cosca-vision-fast-detect` — plugin que recebe frame → retorna objetos detectados (bbox, label, confidence) em <10ms. Input: frame. Output: lista de detecções.

### UTILIDADE NO COSCA
**Média-Alta.** YOLO é o **detector de reação rápida** — GroundingDINO é mais flexível (linguagem), YOLO é mais rápido (tempo real). Podem coexistir: YOLO para monitoramento, GroundingDINO para busca por linguagem.

---

## 6. Depth Anything V2

### REPO
`DepthAnything/Depth-Anything-V2` — ★8.6k — Apache-2.0 — Python

### PAPER/MÉTODO
Depth Anything V2 estima **profundidade monocular** (de uma imagem) com alta qualidade. Usa DINOv2 como backbone + decoder para profundidade. Treinado em dados sintéticos (DIODE, Hypersim) + reais. Resultado: mapa de profundidade de qualquer imagem.

### MODELO
V2-S/M/L/G. Pesos open-source.

### LICENSE
Apache-2.0 ✅

### DEPENDÊNCIAS
PyTorch, DINOv2 (como backbone). Moderado (~500MB GPU).

### BENCHMARK
State-of-the-art em depth estimation (DIODE, NYUv2, KITTI). Muito mais robusto que MiDaS.

### INTEGRAÇÃO NO COSCA
O agente olha → Depth Anything V2 diz "quão longe está cada coisa". É a **percepção de profundidade** — o agente sabe distâncias no mundo. Essencial para: navegação ("está perto ou longe?"), interação ("posso alcançar?"), percepção espacial ("isso está à frente ou atrás?").

### POSSÍVEL PLUGIN
`cosca-vision-depth` — plugin que recebe imagem → retorna mapa de profundidade (float32 por pixel). Input: frame. Output: depth map + distância média/max/min.

### UTILIDADE NO COSCA
**Alta.** Sem profundidade, o agente vê um mundo 2D. Com Depth Anything, ele vê um mundo **3D** — distâncias, proximidade, alcance. Essencial para navegação e interação física.

---

## Síntese da Camada Vision

| # | Projeto | ★ | License | Capacidade no Cosca |
|---|---------|---|---------|---------------------|
| 1 | SAM2 | 19.7k | Apache-2.0 | Segmentar objetos (máscara) |
| 2 | CLIP | 28k | MIT | Conectar visão à linguagem |
| 3 | GroundingDINO | 10.5k | Apache-2.0 | Detectar por query textual |
| 4 | DINOv2 | 13.2k | Apache-2.0 | Features visuais genéricas |
| 5 | YOLO | 60.8k | AGPL-3.0 | Detecção em tempo real |
| 6 | Depth Anything V2 | 8.6k | Apache-2.0 | Profundidade monocular |

### O pipeline visual do agente (visão →Cosca)

```
Frame do mundo
    │
    ├─→ YOLO (rápido, contínuo) → "o que tem aqui?" (alertas)
    │
    ├─→ GroundingDINO (busca) → "onde está o X?" (localização)
    │         │
    │         └─→ SAM2 (segmentação) → "qual a forma exata?" (máscara)
    │
    ├─→ CLIP (classificação) → "isso é o quê?" (nome)
    │
    ├─→ Depth Anything V2 → "quão longe está?" (distância)
    │
    └─→ DINOv2 (features) → "isso se parece com o quê?" (memória visual)
```

### Prioridade de instalação (quando o NVMe chegar)
1. **CLIP** (MIT, leve, mais versátil) — o dicionário visual
2. **SAM2** (Apache, segmentação) — o olho que segmenta
3. **GroundingDINO** (Apache, detecção grounded) — o ponteiro
4. **Depth Anything V2** (Apache, profundidade) — o senso de profundidade
5. **DINOv2** (Apache, features) — o cérebro visual
6. **YOLO** (AGPL, real-time) — o detector rápido

## Related Patterns

- [`mining-map-world-vivo.md`](mining-map-world-vivo.md) — Gap #1 (Vision) é esta camada
- [`unreal-integration-patterns.md`](unreal-integration-patterns.md) — Perception (sentidos do agente no UE)
- [`mega-brain-patterns.md`](mega-brain-patterns.md) — memória visual (DINOv2 como extrator de memória)
