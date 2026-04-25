package musicrefresh

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// Placeholder is the bytes of a generic music-themed PNG used as
// the initial photo when nowplaying-cli reports no artwork (some
// players — podcasts, browser audio — don't expose artworkData).
//
// Generated programmatically at package init so there's no
// separate image file to bundle. The image is a 200x200 deep-
// purple square; small enough (<1 KB) that the binary stays lean
// and Telegram accepts it as a valid sendPhoto payload.
//
// Packaged as a package-level var rather than a constant because
// PNG bytes can't be a Go string constant; treated as immutable
// by every reader.
var Placeholder []byte

func init() {
	// 200x200 flat fill in a music-app-friendly purple.
	const size = 200
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill := color.RGBA{R: 0x8a, G: 0x2b, B: 0xe2, A: 0xff}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	// PNG encoding never errors on a valid image; ignore the error
	// to keep the package init signature clean.
	_ = png.Encode(&buf, img)
	Placeholder = buf.Bytes()
}
