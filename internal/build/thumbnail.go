package build

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

// thumbnailSize matches what the launcher shows in its mod list.
const thumbnailSize = 512

// writePlaceholderThumbnail generates a plain thumbnail so the mod is
// complete and loadable. EU5 requires the picture named in metadata.json to
// exist; it does not care what is in it. Replace the file with real artwork
// and the build will leave it alone.
func writePlaceholderThumbnail(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, thumbnailSize, thumbnailSize))

	bg := color.RGBA{R: 0x1c, G: 0x24, B: 0x33, A: 0xff}
	border := color.RGBA{R: 0xc8, G: 0xa2, B: 0x5a, A: 0xff}
	const inset = 24

	for y := 0; y < thumbnailSize; y++ {
		for x := 0; x < thumbnailSize; x++ {
			c := bg
			onBorder := x < inset || y < inset ||
				x >= thumbnailSize-inset || y >= thumbnailSize-inset
			if onBorder {
				c = border
			}
			img.SetRGBA(x, y, c)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
