# COSCA CREATIVE / SCIENTIFIC / MEDIA ECOSYSTEM

> Doutrina-mestra da plataforma de criação, engenharia, ciência, multimídia e simulação.
> Dono: Cosca Kernel · Fase 0 → Fase 9 COMPLETA · Início: 2026-08-13
>
> **Estado (v1.0):** as 9 fases do manifesto §37 foram entregues — Engine, Editor,
> Image, Cinema, Music, 3D, Game, Scientific e Cross-Product. O §31 (IA→projeto)
> está materializado. O mapa oficial está no [BLUEPRINT v1.0](BLUEPRINT.md).
>
> **Missão (do Don):** transformar o COSCA em uma plataforma de criação assistida por IA sobre um
> mesmo núcleo tecnológico. **Não copiar produtos existentes** — estudar os melhores projetos open
> source, compreender princípios/arquiteturas/algoritmos/trade-offs, e projetar implementações próprias.

## Arquitetura alvo (manifesto §42)

```
                     COSCA
                       |
        +--------------+--------------+
        |              |              |
      CREATE        ENGINEER       DISCOVER
        |              |              |
     IMAGE          CODE          SCIENCE
     VIDEO          SYSTEMS       DATA
     AUDIO          CLOUD         MODELS
     3D             SECURITY      SIMULATION
     GAME           NETWORK       RESEARCH
        |
        +-----------------------------+
                                      |
                                AI ENGINE
                                      |
                              KNOWLEDGE ENGINE
                                      |
                              WORKFLOW ENGINE
                                      |
                               GPU / RUNTIME
                                      |
                                ASSET SYSTEM
                                      |
                                PROJECT SYSTEM
```

## Produtos (fases 3+)

| Produto | Domínio |
|---|---|
| COSCA EDITOR | Interface universal adaptativa (texto/código/imagem/vídeo/áudio/documento/dados/node graph) |
| COSCA IMAGE | Gerar/editar/restaurar/upscale/segmentar/inpaint/compor/vetorizar |
| COSCA CINEMA | Roteiro→storyboard→shot list→cenas→timeline→VFX→cor→áudio→render |
| COSCA MUSIC | Gravação/edição/mixagem/mastering/MIDI/síntese/amostragem/stems |
| COSCA GAME | Engine→projeto→cena→entidade→componente→sistema (2D/3D/física/IA/rede) |
| COSCA SCIENTIFIC | Documento+dados+código+simulação+visualização+modelo (reprodutível) |
| COSCA 3D | Modelagem/materiais/iluminação/câmeras/animação/cena/render (OBJ/FBX/glTF/USD) |
| COSCA ANIMATION | Timeline/keyframes/rig/esqueleto/motion/curvas/física (2D/3D/character/motion graphics) |
| COSCA DOCUMENT | Editor documental inteligente (PDF/DOC/MD/HTML/SVG) com OCR/extração/IA |
| COSCA LAB | Experimentos/modelos/datasets/runs/comparação/A-B/benchmarks/sweeps |

## Engines compartilhadas (Fase 1)

PROJECT · ASSET · AI · AI-TASK · WORKFLOW · PLUGIN · MEDIA · GPU · MODEL · KNOWLEDGE ·
NODE-GRAPH · RENDER · CACHE · VERSION · COLLABORATION · SECURITY · OBSERVABILITY · TASK · RUNTIME

## Documentos da Fase 0

| Documento | Conteúdo |
|---|---|
| [BLUEPRINT.md](BLUEPRINT.md) | Síntese-mestre: princípios dos 64 projetos + contratos das engines + mapeamento do que já existe |
| [research/](research/) | Dossiês completos (Research Matrix, 20 campos por projeto) |
| `research/01-arquitetura-ai.md` | Arquitetura (4) + IA (5) |
| `research/02-imagem.md` | Imagem (6) — ComfyUI node graph em profundidade |
| `research/03-video-audio.md` | Vídeo (7) + Áudio (9) |
| `research/04-3d-game.md` | 3D/Graphics (8) + Game (4) |
| `research/05-scientific-docs-workflow.md` | Científico (7) + Documentos (6) + Workflow (3) + DevTools (5) |

## Regras da casa

1. **ESTUDAR O MUNDO, NÃO COPIAR** — princípios e arquiteturas sim; código alheio, só via licença verificada.
2. **Não reimplementar o que funciona** — o Cosca já tem workflow DAG, providers AI, cache, security, knowledge.
3. **Pipeline IA = grafo de nós avaliado por demanda com cache por assinatura de inputs** (padrão ComfyUI).
4. **Durable execution para tarefas longas resumíveis** (§40 do manifesto — padrão Temporal).
5. **GPU = AMD ROCm (gfx1030)** — validar suporte por projeto antes de prometer aceleração; Vulkan/OpenCL/VAAPI como caminho.
6. **Licenças desde o dia 1** — nada de GPL/AGPL no núcleo proprietário sem plano (PyMuPDF=AGPL, JUCE=GPL, Zed=GPL).
7. **I/O e codecs via FFmpeg** — nunca reescrever parsing de mídia.
8. **Todo input externo é não confiável** — parsers sandboxed (mídia/modelos/plugins/projetos).
9. **Cada produto é um MODO do Editor** — mesma identidade, mesmo engine, mesmas engines compartilhadas.
