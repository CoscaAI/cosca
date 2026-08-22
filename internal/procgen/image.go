package procgen

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// RGBA é a imagem própria do engine — row-major, 4 bytes/pixel (R,G,B,A),
// sem depender de image.RGBA como tipo principal (os nós do Procedural Kernel
// System trocam *RGBA entre si).
type RGBA struct {
	W, H int
	Pix  []uint8
}

// NewRGBA cria uma imagem W×H com pixels zerados (preto transparente).
func NewRGBA(w, h int) *RGBA {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &RGBA{W: w, H: h, Pix: make([]uint8, w*h*4)}
}

// idx devolve o offset do pixel (x,y) ou -1 se fora dos limites.
func (im *RGBA) idx(x, y int) int {
	if x < 0 || y < 0 || x >= im.W || y >= im.H {
		return -1
	}
	return (y*im.W + x) * 4
}

// Set grava o pixel (x,y). Fora dos limites é no-op (fail-closed: nunca
// indexa fora da memória).
func (im *RGBA) Set(x, y int, r, g, b, a uint8) {
	if i := im.idx(x, y); i >= 0 {
		im.Pix[i+0] = r
		im.Pix[i+1] = g
		im.Pix[i+2] = b
		im.Pix[i+3] = a
	}
}

// Get devolve o pixel (x,y); fora dos limites devolve 0,0,0,0.
func (im *RGBA) Get(x, y int) (uint8, uint8, uint8, uint8) {
	if i := im.idx(x, y); i >= 0 {
		return im.Pix[i+0], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3]
	}
	return 0, 0, 0, 0
}

// toImage converte para image.NRGBA da stdlib (não-premultiplicado — o RGBA
// próprio guarda valores crus, como o PNG espera).
func (im *RGBA) toImage() *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, im.W, im.H))
	idx := 0
	for y := 0; y < im.H; y++ {
		for x := 0; x < im.W; x++ {
			dst.SetNRGBA(x, y, color.NRGBA{
				R: im.Pix[idx+0], G: im.Pix[idx+1],
				B: im.Pix[idx+2], A: im.Pix[idx+3],
			})
			idx += 4
		}
	}
	return dst
}

// WritePNG grava a imagem em PNG via image/png.
func (im *RGBA) WritePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("procgen: create png %s: %w", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, im.toImage()); err != nil {
		return fmt.Errorf("procgen: encode png %s: %w", path, err)
	}
	return nil
}

// WritePPM grava a imagem em PPM binário P6 (RGB, sem alpha). O header é
// "P6\n<w> <h>\n255\n".
func (im *RGBA) WritePPM(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("procgen: create ppm %s: %w", path, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "P6\n%d %d\n255\n", im.W, im.H); err != nil {
		return fmt.Errorf("procgen: write ppm header %s: %w", path, err)
	}
	// PPM não tem alpha — escreve só RGB.
	buf := make([]uint8, 0, im.W*im.H*3)
	for i := 0; i < len(im.Pix); i += 4 {
		buf = append(buf, im.Pix[i+0], im.Pix[i+1], im.Pix[i+2])
	}
	if _, err := f.Write(buf); err != nil {
		return fmt.Errorf("procgen: write ppm %s: %w", path, err)
	}
	return nil
}

// Blit copia src sobre dst na posição (dx,dy) com alpha blending (src over
// dst): out = a*src + (1-a)*dst. Fora dos limites é recortado (clipped).
func (im *RGBA) Blit(dst *RGBA, src *RGBA, dx, dy int) {
	if dst == nil || src == nil {
		return
	}
	for sy := 0; sy < src.H; sy++ {
		dyi := dy + sy
		if dyi < 0 || dyi >= dst.H {
			continue
		}
		for sx := 0; sx < src.W; sx++ {
			dxi := dx + sx
			if dxi < 0 || dxi >= dst.W {
				continue
			}
			si := (sy*src.W + sx) * 4
			di := (dyi*dst.W + dxi) * 4
			sr, sg, sb, sa := src.Pix[si], src.Pix[si+1], src.Pix[si+2], src.Pix[si+3]
			dr, dg, db, da := dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3]
			alpha := float64(sa) / 255.0
			dst.Pix[di+0] = uint8(float64(sr)*alpha + float64(dr)*(1-alpha))
			dst.Pix[di+1] = uint8(float64(sg)*alpha + float64(dg)*(1-alpha))
			dst.Pix[di+2] = uint8(float64(sb)*alpha + float64(db)*(1-alpha))
			dst.Pix[di+3] = uint8(float64(sa) + float64(da)*(1-alpha))
		}
	}
}

// Map aplica f por pixel e preenche a imagem. nx,ny são normalizados 0..1
// (nx = x/(W-1)). O retorno [3]float64 é clampado a [0,1] e vira R=G=B;
// alpha fica 255. É a ponte entre pattern/noise functions e a imagem.
func (im *RGBA) Map(f func(x, y int, nx, ny float64) [3]float64) {
	if im.W <= 0 || im.H <= 0 {
		return
	}
	idx := 0
	for y := 0; y < im.H; y++ {
		ny := float64(y)
		if im.H > 1 {
			ny /= float64(im.H - 1)
		}
		for x := 0; x < im.W; x++ {
			nx := float64(x)
			if im.W > 1 {
				nx /= float64(im.W - 1)
			}
			c := f(x, y, nx, ny)
			im.Pix[idx+0] = uint8(Clamp(c[0], 0, 1) * 255)
			im.Pix[idx+1] = uint8(Clamp(c[1], 0, 1) * 255)
			im.Pix[idx+2] = uint8(Clamp(c[2], 0, 1) * 255)
			im.Pix[idx+3] = 255
			idx += 4
		}
	}
}
