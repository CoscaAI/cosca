#!/usr/bin/env python3
"""
COSCA VISUAL MEDIA ENGINE — cosca-media

Ferramenta única do Cosca para processamento de mídia visual.
Escolhe o pipeline pelo CONTEÚDO (não pela extensão) e preserva vetor quando possível.

Uso:
  cosca-media identify <file>                     → detecção de formato (magic) + info
  cosca-media remove-bg <in> <out> [--model M]    → remoção de fundo (rembg, u2net default)
  cosca-media convert <in> <out> [--quality N] [--resize WxH]
  cosca-media resize <in> <out> <W> <H>
  cosca-media pdf text <pdf>
  cosca-media pdf pages <pdf> <out_prefix> [--dpi N] [--fmt png|jpg]
  cosca-media svg optimize <in> <out>
  cosca-media svg to-png <in> <out> [--width N]
  cosca-media vectorize <png> <out.svg> [--threshold N]   → raster→vetor simples (flat)
  cosca-media info <file>                       → alias de identify

Princípios (Stack 18): detectar pelo conteúdo · preservar vetor · não destruir original.
"""
import sys
import os
import json
import struct
import subprocess
import shutil

TOOL_DIR = os.path.dirname(os.path.abspath(__file__))
VENV_PY = os.path.expanduser('~/.cosca/venv-media/bin/python')
NODE_BIN = os.path.join(TOOL_DIR, 'node_modules', '.bin')

# ── Format detection by MAGIC (never trust extension) ───────────────
def detect_format(path: str) -> str:
    with open(path, 'rb') as f:
        head = f.read(16)
    if head[:8] == b'\x89PNG\r\n\x1a\n':
        return 'PNG'
    if head[:2] == b'\xff\xd8':
        return 'JPEG'
    if head[:4] == b'RIFF' and head[8:12] == b'WEBP':
        return 'WEBP'
    if head[:4] == b'GIF8':
        return 'GIF'
    if head[:2] == b'BM':
        return 'BMP'
    if head[:4] == b'%PDF':
        return 'PDF'
    if head[:5] == b'<?xml' or head[:4].lstrip()[:5].lower() == b'<svg':
        return 'SVG'
    if head[:3] == b'AVIF' or (head[4:8] == b'ftyp' and head[8:12] in (b'avif', b'avis')):
        return 'AVIF'
    if head[:4] == b'II*\x00' or head[:4] == b'MM\x00*':
        return 'TIFF'
    return 'DESCONHECIDO'

def image_info(path: str) -> dict:
    try:
        from PIL import Image
        with Image.open(path) as im:
            return {'format': im.format, 'mode': im.mode, 'size': list(im.size),
                    'info_keys': list(im.info.keys())}
    except Exception as e:
        return {'error': str(e)}

def cmd_identify(path: str):
    if not os.path.exists(path):
        return fail(f'arquivo não encontrado: {path}')
    fmt = detect_format(path)
    info = image_info(path)
    print(json.dumps({'detected': fmt, 'by_content': True, 'info': info}, indent=2))

# ── Background removal (rembg) ──────────────────────────────────────
def cmd_remove_bg(src: str, dst: str, model: str = 'u2net', feather: int = 0):
    if not os.path.exists(src):
        return fail(f'arquivo não encontrado: {src}')
    if detect_format(src) == 'PDF':
        return fail('remoção de fundo exige imagem; converta o PDF primeiro (pdf pages)')
    code = (
        "import sys\n"
        "from rembg import remove, new_session\n"
        "from PIL import Image\n"
        f"model = {model!r}\n"
        "session = new_session(model)\n"
        f"inp = Image.open({src!r})\n"
        "out = remove(inp, session=session)\n"
        f"out.save({dst!r})\n"
        "print('done')\n"
    )
    r = subprocess.run([VENV_PY, '-c', code], capture_output=True, text=True)
    if r.returncode != 0:
        return fail('rembg falhou: ' + r.stderr[-500:])
    if feather > 0:
        try:
            import cv2
            import numpy as np
            from PIL import Image as PI
            img = PI.open(dst).convert('RGBA')
            alpha = np.array(img.split()[3])
            alpha = cv2.GaussianBlur(alpha, (0, 0), feather)
            img.putalpha(PI.fromarray(alpha))
            img.save(dst)
            print(f'  + edge refinement (feather {feather}px)')
        except Exception as e:
            print(f'  (feather ignorado: {e})')
    print(f'fundo removido → {dst}')

# ── Conversion / resize (Pillow) ────────────────────────────────────
def cmd_convert(src: str, dst: str, quality: int = 90, resize: str | None = None):
    from PIL import Image
    im = Image.open(src)
    if im.mode in ('RGBA', 'P', 'LA') and dst.lower().endswith(('.jpg', '.jpeg')):
        im = im.convert('RGB')  # JPEG não suporta alpha — perder transparência conscientemente
    if resize:
        w, h = (int(x) for x in resize.lower().split('x'))
        im = im.resize((w, h), Image.LANCZOS)
    save_kw = {}
    if dst.lower().endswith(('.jpg', '.jpeg', '.webp')):
        save_kw['quality'] = quality
    im.save(dst, **save_kw)
    print(f'convertido → {dst} ({detect_format(dst)})')

def cmd_resize(src: str, dst: str, w: int, h: int):
    from PIL import Image
    im = Image.open(src)
    im = im.resize((w, h), Image.LANCZOS)
    im.save(dst)
    print(f'redimensionado → {dst}')

# ── PDF (poppler) ───────────────────────────────────────────────────
def _have(cmd: str) -> bool:
    return shutil.which(cmd) is not None

def cmd_pdf_text(pdf: str):
    if not _have('pdftotext'):
        return fail('pdftotext indisponível (poppler-utils)')
    r = subprocess.run(['pdftotext', pdf, '-'], capture_output=True, text=True)
    print(r.stdout if r.stdout.strip() else '(sem texto extraível — provavelmente PDF escaneado; use OCR)')

def cmd_pdf_pages(pdf: str, prefix: str, dpi: int = 150, fmt: str = 'png'):
    if not _have('pdftoppm'):
        return fail('pdftoppm indisponível (poppler-utils)')
    r = subprocess.run(['pdftoppm', '-r', str(dpi), '-f', '1', '-l', str(10**9), f'-{fmt}', pdf, prefix],
                       capture_output=True, text=True)
    if r.returncode != 0:
        return fail('pdftoppm falhou: ' + r.stderr[-300:])
    outs = sorted(f for f in os.listdir(os.path.dirname(prefix) or '.') if f.startswith(os.path.basename(prefix)))
    print(f'páginas renderizadas: {len(outs)} → {os.path.dirname(os.path.abspath(prefix))}')

# ── PDF via PyMuPDF (preserva objetos — nunca rasterizar sem motivo) ─
def _pm():
    try:
        import pymupdf  # PyMuPDF 1.24+ (fitz renomeado)
        return pymupdf
    except ImportError:
        return fail('pymupdf indisponível (pip install pymupdf)')

def cmd_pdf_text_pm(pdf: str):
    pm = _pm()
    doc = pm.open(pdf)
    total = []
    for page in doc:
        total.append(page.get_text())
    text = '\n'.join(total)
    print(text if text.strip() else '(sem texto extraível — PDF escaneado; use: cosca-media ocr)')

def cmd_pdf_merge(out: str, inputs: list):
    pm = _pm()
    merged = pm.open()
    for path in inputs:
        if not os.path.exists(path):
            return fail(f'PDF não encontrado: {path}')
        with pm.open(path) as src:
            merged.insert_pdf(src)
    merged.save(out)
    print(f'merge: {len(inputs)} PDFs → {out}')

def cmd_pdf_split(pdf: str, prefix: str):
    pm = _pm()
    doc = pm.open(pdf)
    for i, page in enumerate(doc, 1):
        out = f'{prefix}-{i:03d}.pdf'
        new = pm.open()
        new.insert_pdf(doc, from_page=i - 1, to_page=i - 1)
        new.save(out)
        new.close()
    print(f'split: {len(doc)} páginas → {prefix}-NNN.pdf')

def cmd_pdf_extract_images(pdf: str, outdir: str):
    pm = _pm()
    os.makedirs(outdir, exist_ok=True)
    doc = pm.open(pdf)
    count = 0
    for pno, page in enumerate(doc, 1):
        for img in page.get_images(full=True):
            xref = img[0]
            pix = pm.Pixmap(doc, xref)
            if pix.n - pix.alpha > 3:  # CMYK → RGB
                pix = pm.Pixmap(pm.csRGB, pix)
            pix.save(os.path.join(outdir, f'p{pno}-img{count+1}.png'))
            count += 1
    print(f'imagens extraídas: {count} → {outdir}')

# ── OCR (tesseract) ──────────────────────────────────────────────────
def cmd_ocr(src: str, lang: str = 'por'):
    if not _have('tesseract'):
        return fail('tesseract indisponível (sudo apt-get install tesseract-ocr tesseract-ocr-por)')
    fmt = detect_format(src)
    if fmt == 'PDF':
        # render first page(s) to temp PNG, then OCR
        import tempfile
        tmpd = tempfile.mkdtemp()
        prefix = os.path.join(tmpd, 'pg')
        cmd_pdf_pages(src, prefix, dpi=200, fmt='png')
        pngs = sorted(f for f in os.listdir(tmpd) if f.startswith('pg'))
        if not pngs:
            return fail('falha ao renderizar PDF para OCR')
        texts = []
        for p in pngs:
            r = subprocess.run(['tesseract', os.path.join(tmpd, p), 'stdout', '-l', lang],
                               capture_output=True, text=True)
            texts.append(r.stdout)
        import shutil as _sh
        _sh.rmtree(tmpd, ignore_errors=True)
        print('\n'.join(texts))
    else:
        r = subprocess.run(['tesseract', src, 'stdout', '-l', lang], capture_output=True, text=True)
        if r.returncode != 0:
            return fail('tesseract falhou: ' + r.stderr[-300:])
        print(r.stdout)

# ── SVG (svgo + sharp, via node) ────────────────────────────────────
def cmd_svg_optimize(src: str, dst: str):
    svgo = os.path.join(NODE_BIN, 'svgo')
    if not os.path.exists(svgo):
        return fail('svgo indisponível (rode: cd scripts && npm install svgo)')
    r = subprocess.run([svgo, '--input', src, '--output', dst], capture_output=True, text=True)
    if r.returncode != 0:
        return fail('svgo falhou: ' + r.stderr[-300:])
    print(f'svg otimizado → {dst}')

def cmd_svg_to_png(src: str, dst: str, width: int = 512):
    sharp_module = os.path.join(TOOL_DIR, 'node_modules', 'sharp')
    if not os.path.exists(os.path.join(sharp_module, 'package.json')):
        return fail('sharp indisponível (cd scripts && npm install sharp)')
    js = (
        f"const sharp = require({sharp_module!r});\n"
        f"sharp({src!r}).resize({width}, null).png().toFile({dst!r})\n"
        f".then(()=>console.log('done')).catch(e=>{{console.error(e.message);process.exit(1)}})\n"
    )
    r = subprocess.run(['node', '-e', js], capture_output=True, text=True)
    if r.returncode != 0:
        return fail('sharp falhou: ' + r.stderr[-300:])
    print(f'svg renderizado → {dst}')

# ── Raster → vector (potrace real, com fallback simples) ────────────
def cmd_vectorize(src: str, dst: str, threshold: int = 128):
    if shutil.which('potrace'):
        return _vectorize_potrace(src, dst, threshold)
    return _vectorize_simple(src, dst, threshold)

def _vectorize_potrace(src: str, dst: str, threshold: int):
    """Potrace: raster → Bézier paths de verdade (logos/ícones)."""
    import tempfile
    from PIL import Image, ImageOps
    im = Image.open(src)
    if im.mode in ('RGBA', 'LA', 'P'):
        im = im.convert('RGBA')
        bg = Image.new('RGBA', im.size, (255, 255, 255, 255))
        im = Image.alpha_composite(bg, im)
    im = im.convert('L')
    im = ImageOps.autocontrast(im)
    bw = im.point(lambda p: 255 if p < threshold else 0)
    tmp = tempfile.NamedTemporaryFile(suffix='.bmp', delete=False)
    try:
        bw.save(tmp.name)
        r = subprocess.run(['potrace', '-s', '-o', dst, tmp.name], capture_output=True, text=True)
        if r.returncode != 0:
            return fail('potrace falhou: ' + r.stderr[-300:])
    finally:
        os.unlink(tmp.name)
    print(f'vectorizado (potrace) → {dst}')

def _vectorize_simple(src: str, dst: str, threshold: int):
    """Convert image to an editable SVG via threshold + connected-component paths.
    Adequado para logos/ícones flat; não para fotografias."""
    from PIL import Image, ImageOps
    im = Image.open(src)
    # Compose over white to handle transparency correctly
    if im.mode in ('RGBA', 'LA', 'P'):
        im = im.convert('RGBA')
        bg = Image.new('RGBA', im.size, (255, 255, 255, 255))
        im = Image.alpha_composite(bg, im)
    im = im.convert('L')
    im = ImageOps.autocontrast(im)
    # Foreground = pixels MAIS ESCUROS que o threshold (logo escuro em fundo claro)
    bw = im.point(lambda p: 255 if p < threshold else 0)
    px = bw.load()
    w, h = bw.size
    visited = set()
    components = []
    for y in range(h):
        for x in range(w):
            if (x, y) in visited or px[x, y] != 255:
                continue
            # BFS connected component
            stack = [(x, y)]
            visited.add((x, y))
            pts = []
            while stack:
                cx, cy = stack.pop()
                pts.append((cx, cy))
                for dx, dy in ((1,0),(-1,0),(0,1),(0,-1)):
                    nx, ny = cx+dx, cy+dy
                    if 0 <= nx < w and 0 <= ny < h and (nx, ny) not in visited and px[nx, ny] == 255:
                        visited.add((nx, ny))
                        stack.append((nx, ny))
            if len(pts) >= 8:  # ignore noise
                components.append(pts)
    parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}">']
    for pts in components:
        # bounding box per component → rect (simples) ; contours → path para formas únicas
        xs = [p[0] for p in pts]; ys = [p[1] for p in pts]
        parts.append(
            f'<rect x="{min(xs)}" y="{min(ys)}" width="{max(xs)-min(xs)+1}" height="{max(ys)-min(ys)+1}" fill="#000000"/>'
        )
    parts.append('</svg>')
    with open(dst, 'w') as f:
        f.write('\n'.join(parts))
    print(f'vectorizado → {dst} ({len(components)} formas)')

def fail(msg: str):
    print(f'ERRO: {msg}', file=sys.stderr)
    sys.exit(1)

USAGE = __doc__

def main():
    args = sys.argv[1:]
    if not args:
        print(USAGE); sys.exit(0)
    cmd = args[0]
    try:
        if cmd in ('identify', 'info'):
            if len(args) < 2: return fail('uso: cosca-media identify <file>')
            cmd_identify(args[1])
        elif cmd == 'remove-bg':
            if len(args) < 3: return fail('uso: remove-bg <in> <out> [--model u2net] [--feather N]')
            model, feather = 'u2net', 0
            if '--model' in args: model = args[args.index('--model')+1]
            if '--feather' in args: feather = int(args[args.index('--feather')+1])
            cmd_remove_bg(args[1], args[2], model, feather)
        elif cmd == 'convert':
            if len(args) < 3: return fail('uso: convert <in> <out> [--quality 90] [--resize WxH]')
            quality, resize = 90, None
            if '--quality' in args: quality = int(args[args.index('--quality')+1])
            if '--resize' in args: resize = args[args.index('--resize')+1]
            cmd_convert(args[1], args[2], quality, resize)
        elif cmd == 'resize':
            if len(args) < 5: return fail('uso: resize <in> <out> <W> <H>')
            cmd_resize(args[1], args[2], int(args[3]), int(args[4]))
        elif cmd == 'pdf':
            sub = args[1] if len(args) > 1 else ''
            if sub == 'text': cmd_pdf_text_pm(args[2])
            elif sub == 'pages':
                dpi, fmt = 150, 'png'
                if '--dpi' in args: dpi = int(args[args.index('--dpi')+1])
                if '--fmt' in args: fmt = args[args.index('--fmt')+1]
                cmd_pdf_pages(args[2], args[3], dpi, fmt)
            elif sub == 'merge': cmd_pdf_merge(args[2], args[3:])
            elif sub == 'split': cmd_pdf_split(args[2], args[3])
            elif sub == 'images': cmd_pdf_extract_images(args[2], args[3])
            else: return fail('uso: pdf text|pages|merge|split|images ...')
        elif cmd == 'ocr':
            if len(args) < 2: return fail('uso: ocr <image|pdf> [--lang por]')
            lang = 'por'
            if '--lang' in args: lang = args[args.index('--lang')+1]
            cmd_ocr(args[1], lang)
        elif cmd == 'svg':
            sub = args[1] if len(args) > 1 else ''
            if sub == 'optimize': cmd_svg_optimize(args[2], args[3])
            elif sub == 'to-png':
                width = 512
                if '--width' in args: width = int(args[args.index('--width')+1])
                cmd_svg_to_png(args[2], args[3], width)
            else: return fail('uso: svg optimize|to-png ...')
        elif cmd == 'vectorize':
            threshold = 128
            if '--threshold' in args: threshold = int(args[args.index('--threshold')+1])
            cmd_vectorize(args[1], args[2], threshold)
        else:
            print(USAGE); sys.exit(1)
    except Exception as e:
        fail(f'{type(e).__name__}: {e}')

if __name__ == '__main__':
    main()
