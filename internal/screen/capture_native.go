package screen

import (
	"image"

	"github.com/kbinani/screenshot"
)

// numDisplays retorna quantos monitores ativos existem.
func numDisplays() int {
	return screenshot.NumActiveDisplays()
}

// displayBounds retorna a área (bounds) do monitor `display` em coordenadas
// absolutas de tela.
func displayBounds(display int) image.Rectangle {
	return screenshot.GetDisplayBounds(display)
}

// captureDisplay captura a tela inteira do monitor `display`.
func captureDisplay(bounds image.Rectangle) image.Image {
	img, err := screenshot.Capture(bounds.Min.X, bounds.Min.Y, bounds.Dx(), bounds.Dy())
	if err != nil {
		return nil
	}
	return img
}
