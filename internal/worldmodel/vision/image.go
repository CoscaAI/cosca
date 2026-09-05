package vision

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"

	"github.com/yalue/onnxruntime_go"
)

// ──────────────────────────────────────────────────────────────
// Preprocessing
// ──────────────────────────────────────────────────────────────

// decodeImage decodes PNG or JPEG bytes into an image.Image.
func decodeImage(data []byte) (image.Image, error) {
	if img, err := png.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	return nil, fmt.Errorf("unsupported image format (expected PNG or JPEG)")
}

// imageSize returns the decoded image's pixel width and height (0,0 on decode
// failure). Used to scale detector boxes from normalized to pixel coordinates.
func imageSize(data []byte) (int, int) {
	img, err := decodeImage(data)
	if err != nil {
		return 0, 0
	}
	b := img.Bounds()
	return b.Dx(), b.Dy()
}

// resizeBilinear resizes img to width×height using bilinear interpolation.
func resizeBilinear(img image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	b := img.Bounds()
	sw := float64(b.Dx())
	sh := float64(b.Dy())
	if sw == 0 || sh == 0 {
		return dst
	}
	for y := 0; y < height; y++ {
		fy := (float64(y)+0.5)*sh/float64(height) - 0.5
		y0 := int(math.Floor(fy))
		dy := fy - float64(y0)
		for x := 0; x < width; x++ {
			fx := (float64(x)+0.5)*sw/float64(width) - 0.5
			x0 := int(math.Floor(fx))
			dx := fx - float64(x0)
			dst.Set(x, y, bilinearSample(img, x0, y0, dx, dy))
		}
	}
	return dst
}

func bilinearSample(img image.Image, x0, y0 int, dx, dy float64) color.RGBA {
	get := func(x, y int) [4]float64 {
		b := img.Bounds()
		if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
			return [4]float64{}
		}
		r, g, bl, a := img.At(x, y).RGBA()
		return [4]float64{float64(r >> 8), float64(g >> 8), float64(bl >> 8), float64(a >> 8)}
	}
	c00 := get(x0, y0)
	c10 := get(x0+1, y0)
	c01 := get(x0, y0+1)
	c11 := get(x0+1, y0+1)
	var out [4]uint8
	for i := 0; i < 4; i++ {
		top := c00[i]*(1-dx) + c10[i]*dx
		bot := c01[i]*(1-dx) + c11[i]*dx
		v := top*(1-dy) + bot*dy
		out[i] = uint8(math.Max(0, math.Min(255, v)))
	}
	return color.RGBA{out[0], out[1], out[2], out[3]}
}

// normalizeParams holds the per-channel mean/std used to normalise an image.
type normalizeParams struct {
	Mean [3]float32
	Std  [3]float32
}

// imageNetNormalize matches the standard ImageNet normalisation (resnet / depth
// backbones). Used by DepthAnythingV2 and GroundingDINO.
var imageNetNormalize = normalizeParams{
	Mean: [3]float32{0.485, 0.456, 0.406},
	Std:  [3]float32{0.229, 0.224, 0.225},
}

// clipNormalize matches OpenAI CLIP's normalisation.
var clipNormalize = normalizeParams{
	Mean: [3]float32{0.48145466, 0.4578275, 0.40821073},
	Std:  [3]float32{0.26862954, 0.26130258, 0.27577711},
}

// toCHWFloat converts an image to a normalised CHW float32 buffer (single
// image, batch dimension omitted; the caller may wrap it as [1,3,H,W]).
func toCHWFloat(img image.Image, width, height int, p normalizeParams) []float32 {
	src := resizeBilinear(img, width, height)
	n := width * height
	out := make([]float32, 3*n)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			idx := y*width + x
			out[idx] = (float32(r>>8)/255 - p.Mean[0]) / p.Std[0]
			out[n+idx] = (float32(g>>8)/255 - p.Mean[1]) / p.Std[1]
			out[2*n+idx] = (float32(b>>8)/255 - p.Mean[2]) / p.Std[2]
		}
	}
	return out
}

// buildImageTensor decodes the frame, resizes to width×height, normalises and
// wraps it as an NCHW float32 tensor of shape [1,3,height,width] for ONNX.
func buildImageTensor(frame []byte, width, height int, p normalizeParams) (*onnxruntime_go.Tensor[float32], error) {
	img, err := decodeImage(frame)
	if err != nil {
		return nil, err
	}
	data := toCHWFloat(img, width, height, p)
	return onnxruntime_go.NewTensor[float32](
		onnxruntime_go.NewShape(1, 3, int64(height), int64(width)), data)
}

// l2norm normalises a float32 vector in place and returns its original norm.
func l2norm(v []float32) float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	if norm < 1e-9 {
		return 0
	}
	for i := range v {
		v[i] /= norm
	}
	return norm
}

// softmax computes the softmax over logits in place and returns them.
func softmax(logits []float32) []float32 {
	var max float32
	for _, l := range logits {
		if l > max {
			max = l
		}
	}
	var sum float64
	for i, l := range logits {
		e := float32(math.Exp(float64(l - max)))
		logits[i] = e
		sum += float64(e)
	}
	for i := range logits {
		logits[i] = float32(float64(logits[i]) / sum)
	}
	return logits
}
