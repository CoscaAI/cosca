# VISUAL MEDIA ENGINE — cosca-media

> Skill executável do Stack 18 (Visual Media / Image / PDF / SVG / OCR Engineering).
> Uma Visual Media Engine: detecta o tipo pelo CONTEÚDO, escolhe o pipeline e preserva vetor.

## Ativação

A ferramenta única do Cosca é o comando global:

```bash
cosca-media <comando> [args]
```

## Comandos

| Comando | Função | Pipeline |
|---|---|---|
| `cosca-media identify <file>` | detecta formato pelo MAGIC + info (Pillow) | detect-by-content |
| `cosca-media remove-bg <in> <out> [--model u2net]` | remoção de fundo (rembg/ONNX) | segment → alpha → edge |
| `cosca-media convert <in> <out> [--quality N] [--resize WxH]` | conversão de formato (Pillow) | format-aware |
| `cosca-media resize <in> <out> <W> <H>` | redimensiona (LANCZOS) | resize |
| `cosca-media pdf text <pdf>` | extrai texto (pdftotext) | pdf-parse |
| `cosca-media pdf pages <pdf> <prefix> [--dpi N] [--fmt png\|jpg]` | renderiza páginas (pdftoppm) | pdf→image |
| `cosca-media svg optimize <in> <out>` | otimiza SVG (svgo) | svg-minify |
| `cosca-media svg to-png <in> <out> [--width N]` | renderiza SVG→PNG (sharp) | svg→raster |
| `cosca-media vectorize <png> <out.svg> [--threshold N]` | raster→vetor flat (componentes) | raster→vector |

## Regras de uso (Stack 18)

1. **Detectar pelo conteúdo**, nunca pela extensão — `identify` usa MAGIC header.
2. **Nunca rasterizar o que pode ser vetor**: PDF com vetores → `pdf pages` só quando o objetivo é imagem; SVG → otimizar/renderizar, não "editar como imagem".
3. **Não destruir o original**: toda operação grava em novo output.
4. **Formato-aware**: JPEG não suporta alpha — conversão RGBA→JPEG perde transparência conscientemente (e avisa).
5. **Mídia é input não confiável** (Stack 17): processar com limites; SVG nunca executar scripts embutidos.
6. **Human-in-the-loop**: o resultado da IA é sugestão — o usuário revisa/corrige.

## Decisão de pipeline (resumo)

| Conteúdo | Ferramenta |
|---|---|
| Imagem simples | `vectorize` / `convert` |
| Produto / pessoa / geral | `remove-bg` (rembg u2net) |
| SVG | `svg optimize` / `svg to-png` |
| PDF texto | `pdf text` |
| PDF escaneado → imagem | `pdf pages` (+ OCR futuro) |

## Dependências (instaladas em ~/.cosca/venv-media e scripts/node_modules)

- Python: rembg (ONNX/u2net) · Pillow
- Node: svgo · sharp
- Sistema: poppler-utils (pdftotext/pdftoppm)

> OCR (tesseract) e restauração/upscale (Real-ESRGAN) são próximas extensões — o pipeline já reserva a posição.

## Exemplos

```bash
cosca-media identify produto.png
cosca-media remove-bg produto.png produto-nobg.png
cosca-media convert produto-nobg.png produto.webp --quality 85
cosca-media svg optimize logo.svg logo-min.svg
cosca-media svg to-png logo.svg logo.png --width 256
cosca-media vectorize logo-png.png logo.svg
cosca-media pdf pages doc.pdf pagina --dpi 150
```
