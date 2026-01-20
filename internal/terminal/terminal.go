package terminal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"

	"github.com/nfnt/resize"
)

type Capabilities struct {
	SupportsKittyGraphics bool
	SupportsRGB           bool
	TermProgram           string
}

func DetectCapabilities() *Capabilities {
	caps := &Capabilities{
		SupportsRGB: true,
	}

	termProgram := os.Getenv("TERM_PROGRAM")
	useKittyGraphics := os.Getenv("LYRECHO_USE_KITTY_GRAPHICS")

	caps.TermProgram = termProgram

	// kitty graphics protocol is opt-in only via environment variable
	if useKittyGraphics != "" {
		switch useKittyGraphics {
		case "1", "true", "yes", "on":
			caps.SupportsKittyGraphics = true
			if caps.TermProgram == "" {
				caps.TermProgram = "kitty"
			}
		case "0", "false", "no", "off":
			caps.SupportsKittyGraphics = false
		}
	}

	return caps
}

func Reset() {
	os.Stdout.WriteString("\033[?25h")
	os.Stdout.WriteString("\033[0m")
	os.Stdout.WriteString("\033[?1049l")
	os.Stdout.WriteString("\033[?1000l")
	os.Stdout.WriteString("\033[?1002l")
	os.Stdout.WriteString("\033[?1003l")
	os.Stdout.WriteString("\033[?1006l")
	os.Stdout.Sync()
}

func EncodeImageForKitty(img image.Image, cols int, rows int) string {
	if img == nil {
		return ""
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width == 0 || height == 0 {
		return ""
	}

	newWidth := uint(cols * 10)
	newHeight := uint(rows * 20)

	aspectRatio := float64(width) / float64(height)
	targetAspect := float64(newWidth) / float64(newHeight)

	if aspectRatio > targetAspect {
		newHeight = uint(float64(newWidth) / aspectRatio)
	} else {
		newWidth = uint(float64(newHeight) * aspectRatio)
	}

	if newWidth < 10 {
		newWidth = 10
	}
	if newHeight < 10 {
		newHeight = 10
	}

	resized := resize.Resize(newWidth, newHeight, img, resize.Lanczos3)

	var buf bytes.Buffer
	err := png.Encode(&buf, resized)
	if err != nil {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	var result strings.Builder

	chunkSize := 4096
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		more := 1
		if end >= len(encoded) {
			more = 0
		}

		if i == 0 {
			result.WriteString(fmt.Sprintf("\x1b_Ga=T,f=100,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, chunk))
		} else {
			result.WriteString(fmt.Sprintf("\x1b_Gm=%d;%s\x1b\\", more, chunk))
		}
	}

	return result.String()
}
