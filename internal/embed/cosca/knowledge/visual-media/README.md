# 18 — VISUAL MEDIA / IMAGE / PDF / SVG / OCR ENGINEERING

> Stack 18 da Cosca Engineering Intelligence Matrix.
> Objetivo: **Visual Media Engineering Intelligence** — entender o arquivo, escolher o pipeline pelo conteúdo, processar, validar e preservar.

## PRINCÍPIO FUNDAMENTAL
```
UNDERSTAND → PLAN → PROCESS → VALIDATE → PRESERVE
```
- Não destruir informação desnecessariamente.
- Não rasterizar vetores sem necessidade.
- Não perder qualidade sem motivo.
- Não confiar cegamente em modelos de IA.
- Não executar conteúdo potencialmente perigoso.
- Não modificar o original destrutivamente.

## VISUAL MEDIA ENGINE (pipeline mestre)
```
COSCA
 → MEDIA ANALYZER (formato detectado pelo CONTEÚDO, não extensão/MIME)
 → CONTENT ANALYZER (fotografia? produto? pessoa? logo? documento? texto?)
 → TASK CLASSIFIER (qual objetivo do usuário?)
 → PIPELINE SELECTOR (qual técnica? qual modelo? qual formato de saída?)
 → PROCESSING ENGINE
 → QUALITY VALIDATOR
 → OUTPUT OPTIMIZER (formato que PRESERVA mais informação)
```

## BACKGROUND REMOVAL — DECISION TREE
| Conteúdo | Pipeline |
|---|---|
| Imagem simples | segmentação clássica / OpenCV (contours, threshold, morphology) |
| Imagem de produto | RMBG / BiRefNet |
| Pessoa | segmentação + matting |
| Cabelo | matting de alta qualidade + hair refinement |
| Objeto específico | SAM / SAM2 / SAM3 |
| Objeto definido por texto | GroundingDINO → SAM |
| Imagem complexa | segmentação → matting → refinement |
| Usuário quer corrigir | **interactive mask editor** |

**Regra**: NÃO existe um algoritmo perfeito para todas as imagens — o pipeline é escolhido pelo conteúdo.

## HUMAN-IN-THE-LOOP (UX de edição)
Nunca obrigar o usuário a aceitar cegamente o resultado da IA. Permitir: `brush · eraser · add mask · remove mask · feather · expand · contract · smooth · invert · edge refinement`.
```
AI → RESULT → USER REVIEW → CORRECTION → FINAL
```
Princípio: **"IA faz 95% → humano corrige os 5% difíceis."**

## IMAGE QUALITY / RESTORATION
Pipelines de qualidade (antes de "melhorar"):
1. **Quality analysis** — detectar: noise, blur, compression artifacts, resolution.
2. Selecionar: denoise? deblur? upscale? face restoration? color correction?
3. Evitar processamento excessivo ("sem deixar artificial").

Referências de estudo: Real-ESRGAN · BasicSR · GFPGAN · CodeFormer · BasicVSR++ · IQA-PyTorch.

## PDF ENGINEERING
- **Parser primeiro, render depois**: PDF object model (text / images / vectors / metadata / fonts / forms / annotations / links / signatures).
- **Preservar objetos**: trabalhar no objeto original; só rasterizar página quando o objetivo é imagem.
- **PDF → imagem**: MuPDF/Poppler/PDFium — render com DPI/color/alpha controlados.
- **Imagem → PDF pesquisável**: OCRmyPDF — render → OCR → text layer → searchable PDF/PDF-A.
- **Operações estruturais**: qpdf (merge/split/repair/linearization/encryption/object inspection).

## OCR / DOCUMENT AI
- OCR: Tesseract · PaddleOCR · Surya · DocTR · EasyOCR — texto, layout, tabelas, formulários, rotação, confidence + bounding boxes.
- Document AI: markitdown · docling · unstructured · OmniParser — PDF → estrutura (tabelas/headings/imagens) → Markdown/JSON/structured.
- PDF escaneado → OCR → texto → pesquisável.

## SVG ENGINEERING
- SVG é **código/vetor**, não "imagem": paths, Bézier, shapes, text, gradients, masks, clipping, filters, transforms, viewBox, coordinate systems.
- **Nunca rasterizar SVG se o objetivo é editar** — editar o DOM/paths.
- Otimização: SVGO (minify, remover metadata, otimizar paths/attributes).
- Renderização: resvg/usvg · librsvg · tiny-skia.

## RASTER → VECTOR
```
PNG → segmentation → threshold → contours → curve fitting → Bezier paths → SVG
```
**Diferenciar e escolher pipeline**: logo simples · ilustração complexa · fotografia · line art · ícone · texto. (Potrace · vtracer · autotrace)
**"Transforme esse logo PNG em SVG editável"** — depois SVGO.

## FORMAT CONVERSION ENGINE
`IMAGE→IMAGE · IMAGE→SVG · IMAGE→PDF · PDF→IMAGE · PDF→SVG (quando tecnicamente possível) · SVG→PNG · SVG→PDF · SVG→WEBP`
**Nunca prometer conversão perfeita** quando o formato não suporta semanticamente os mesmos recursos (ex.: gradiente→PNG é raster; texto→path muda editabilidade).

## LOSSLESS + NON-DESTRUCTIVE
- **Lossless**: preservar original. `INPUT → COPY → PROCESS → OUTPUT`. Metadata só quando apropriado.
- **Non-destructive**: `ORIGINAL + OPERATIONS + MASK + PARAMETERS` em vez de destruir o original.
- **Consciência de codec**: PNG/JPEG/WebP/AVIF/JPEG XL/TIFF/GIF — trade-offs de qualidade/transparência/tamanho; libvips/Sharp para processamento eficiente (memória/streaming/batch/huge images).

## MEDIA SECURITY (input não confiável — cross-ref Stack 17)
- Decompression bombs · oversized images · malformed PDFs · path traversal · resource exhaustion.
- **SVG = conteúdo potencialmente perigoso**: scripts, external references, remote resources, embedded HTML, event handlers, entity expansion (XXE). **Nunca executar scripts embutidos**; sanitizar via parsing estruturado (SVGO/librsvg/resvg como referência de sanitização).
- **Format detection pelo conteúdo real** (file/libmagic), nunca só extensão/MIME declarado.
- Sandboxing ao processar arquivos não confiáveis.

## COMPUTER VISION & RENDERING (base)
- OpenCV/Detectron2/MMDetection/ultralytics: detecção, segmentação, contours, morphology, threshold, edge, color spaces, geometria.
- Skia/FreeType/HarfBuzz/tiny-skia: render 2D, texto, glyphs, anti-aliasing, compositing.
- lcms/OpenColorIO: gestão de cor (RGB/CMYK/Lab, ICC, gamma, HDR).
- FFmpeg/GStreamer: mídia/vídeo (codecs, containers, frame extraction, thumbnails).

## MEDIA KNOWLEDGE TAXONOMY (organização interna)
`image/ · background-removal/ · segmentation/ · matting/ · inpainting/ · restoration/ · upscaling/ · compression/ · color/ · metadata/ · ocr/ · pdf/ · svg/ · vectorization/ · rendering/ · fonts/ · security/ · formats/ · codecs/ · pipelines/ · ux/ · anti-patterns/`

## ANTI-PATTERNS
`rasterizar vetor para editar` · `um único modelo para tudo` · `remover fundo sem edge refinement (halo/serrilhado)` · `upscale ingênuo` · `tratar SVG como imagem` · `confiar em extensão/MIME` · `executar script de SVG/PDF malicioso` · `destruir o original` · `prometer conversão perfeita impossível` · `processar arquivo gigante sem streaming/sandbox` · `OCR sem idioma/qualidade` · `aceitar output sem validação (alpha/bordas/artefatos)`

## CHECKLIST
- [ ] Formato detectado pelo conteúdo (não extensão)
- [ ] Pipeline escolhido pelo conteúdo (decision tree)
- [ ] Vetor preservado quando a fonte tem vetores
- [ ] Edge refinement + decontamination no matting
- [ ] Validar: dimensions, alpha, transparência, halos, color spill, artefatos, corrupção, metadata
- [ ] Formato de saída preserva o máximo de informação
- [ ] Original intacto (não destrutivo) + metadata apropriada
- [ ] Arquivo não confiável: sanitização + sandbox + limits
- [ ] Human-in-the-loop quando o usuário precisa corrigir

## A REGRA
O Cosca não "edita imagens" — ele compreende **o que é o arquivo, o que existe dentro dele, o objetivo do usuário, a melhor representação, o melhor algoritmo, o melhor pipeline, como validar, e qual formato preserva mais informação**.
`UNDERSTAND → PLAN → PROCESS → VALIDATE → PRESERVE`

## REFERÊNCIAS
**Remoção/matting**: rembg · RMBG-2.0 · BiRefNet · InSPyReNet · ComfyUI-RMBG · SAM/SAM2 · GroundingDINO · IOPaint · Interactive-Image-Background-Remover
**Visão**: OpenCV · ultralytics · Detectron2 · MMDetection · MMSegmentation
**Processamento**: libvips · ImageMagick · Pillow · Sharp · libjxl · libwebp · libavif
**Restauração**: Real-ESRGAN · BasicSR · GFPGAN · CodeFormer · IQA-PyTorch
**PDF**: MuPDF · Poppler · PDFium · qpdf · OCRmyPDF · pypdf · PyMuPDF · pdfplumber · WeasyPrint
**OCR/DocAI**: Tesseract · PaddleOCR · Surya · DocTR · EasyOCR · markitdown · docling · unstructured · OmniParser
**SVG/Vector**: resvg/usvg · librsvg · SVGO · tiny-skia · Potrace · vtracer · Inkscape
**Render/cor**: Skia · FreeType · HarfBuzz · lcms · OpenColorIO
**Mídia**: FFmpeg · GStreamer · exiv2 · ExifTool
**IA/geral**: ComfyUI · diffusers · transformers
