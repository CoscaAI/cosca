package screen

import (
	"bytes"
	"fmt"
	"image"
)

// byteBuffer é um bytes.Buffer reutilizável para codificar imagens.
type byteBuffer struct {
	bytes.Buffer
}

// Bytes devolve o slice de bytes acumulado.
func (b *byteBuffer) Bytes() []byte { return b.Buffer.Bytes() }

// grabDisplay captura a tela do monitor `display` (0 = primário) usando o
// backend nativo kbinani/screenshot (GDI no Windows, zero binário externo).
func grabDisplay(display int) (image.Image, error) {
	// NumActiveDisplays() reporta quantos monitores existem.
	if display < 0 {
		display = 0
	}
	if display >= numDisplays() {
		return nil, fmt.Errorf("display %d fora do intervalo (0..%d)", display, numDisplays()-1)
	}
	bounds := displayBounds(display)
	img := captureDisplay(bounds)
	if img == nil {
		return nil, fmt.Errorf("captura de tela falhou (display %d)", display)
	}
	return img, nil
}
