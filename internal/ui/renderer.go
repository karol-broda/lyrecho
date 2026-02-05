package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"karolbroda.com/lyrecho/internal/artwork"
	"karolbroda.com/lyrecho/internal/colors"
)

var pixelFont = map[rune][5]uint8{
	'A': {0b01110, 0b10001, 0b11111, 0b10001, 0b10001},
	'B': {0b11110, 0b10001, 0b11110, 0b10001, 0b11110},
	'C': {0b01111, 0b10000, 0b10000, 0b10000, 0b01111},
	'D': {0b11110, 0b10001, 0b10001, 0b10001, 0b11110},
	'E': {0b11111, 0b10000, 0b11110, 0b10000, 0b11111},
	'F': {0b11111, 0b10000, 0b11110, 0b10000, 0b10000},
	'G': {0b01111, 0b10000, 0b10011, 0b10001, 0b01110},
	'H': {0b10001, 0b10001, 0b11111, 0b10001, 0b10001},
	'I': {0b11111, 0b00100, 0b00100, 0b00100, 0b11111},
	'J': {0b00111, 0b00001, 0b00001, 0b10001, 0b01110},
	'K': {0b10001, 0b10010, 0b11100, 0b10010, 0b10001},
	'L': {0b10000, 0b10000, 0b10000, 0b10000, 0b11111},
	'M': {0b10001, 0b11011, 0b10101, 0b10001, 0b10001},
	'N': {0b10001, 0b11001, 0b10101, 0b10011, 0b10001},
	'O': {0b01110, 0b10001, 0b10001, 0b10001, 0b01110},
	'P': {0b11110, 0b10001, 0b11110, 0b10000, 0b10000},
	'Q': {0b01110, 0b10001, 0b10101, 0b10010, 0b01101},
	'R': {0b11110, 0b10001, 0b11110, 0b10010, 0b10001},
	'S': {0b01111, 0b10000, 0b01110, 0b00001, 0b11110},
	'T': {0b11111, 0b00100, 0b00100, 0b00100, 0b00100},
	'U': {0b10001, 0b10001, 0b10001, 0b10001, 0b01110},
	'V': {0b10001, 0b10001, 0b10001, 0b01010, 0b00100},
	'W': {0b10001, 0b10001, 0b10101, 0b11011, 0b10001},
	'X': {0b10001, 0b01010, 0b00100, 0b01010, 0b10001},
	'Y': {0b10001, 0b01010, 0b00100, 0b00100, 0b00100},
	'Z': {0b11111, 0b00010, 0b00100, 0b01000, 0b11111},

	'a': {0b00000, 0b01110, 0b00001, 0b01111, 0b01111},
	'b': {0b10000, 0b10000, 0b11110, 0b10001, 0b11110},
	'c': {0b00000, 0b01110, 0b10000, 0b10000, 0b01110},
	'd': {0b00001, 0b00001, 0b01111, 0b10001, 0b01111},
	'e': {0b01110, 0b10001, 0b11111, 0b10000, 0b01110},
	'f': {0b00110, 0b01000, 0b11110, 0b01000, 0b01000},
	'g': {0b01111, 0b10001, 0b01111, 0b00001, 0b01110},
	'h': {0b10000, 0b10000, 0b11110, 0b10001, 0b10001},
	'i': {0b00100, 0b00000, 0b00100, 0b00100, 0b00100},
	'j': {0b00010, 0b00000, 0b00010, 0b00010, 0b01100},
	'k': {0b10000, 0b10010, 0b11100, 0b10010, 0b10001},
	'l': {0b01100, 0b00100, 0b00100, 0b00100, 0b01110},
	'm': {0b00000, 0b11010, 0b10101, 0b10101, 0b10001},
	'n': {0b00000, 0b11110, 0b10001, 0b10001, 0b10001},
	'o': {0b00000, 0b01110, 0b10001, 0b10001, 0b01110},
	'p': {0b00000, 0b11110, 0b10001, 0b11110, 0b10000},
	'q': {0b00000, 0b01111, 0b10001, 0b01111, 0b00001},
	'r': {0b00000, 0b10110, 0b11000, 0b10000, 0b10000},
	's': {0b00000, 0b01110, 0b11000, 0b00110, 0b11100},
	't': {0b01000, 0b11110, 0b01000, 0b01000, 0b00110},
	'u': {0b00000, 0b10001, 0b10001, 0b10001, 0b01110},
	'v': {0b00000, 0b10001, 0b10001, 0b01010, 0b00100},
	'w': {0b00000, 0b10001, 0b10101, 0b10101, 0b01010},
	'x': {0b00000, 0b10001, 0b01010, 0b01010, 0b10001},
	'y': {0b00000, 0b10001, 0b01111, 0b00001, 0b01110},
	'z': {0b00000, 0b11111, 0b00110, 0b01100, 0b11111},

	'0': {0b01110, 0b10011, 0b10101, 0b11001, 0b01110},
	'1': {0b00100, 0b01100, 0b00100, 0b00100, 0b01110},
	'2': {0b01110, 0b10001, 0b00110, 0b01000, 0b11111},
	'3': {0b11110, 0b00001, 0b00110, 0b00001, 0b11110},
	'4': {0b10001, 0b10001, 0b11111, 0b00001, 0b00001},
	'5': {0b11111, 0b10000, 0b11110, 0b00001, 0b11110},
	'6': {0b01110, 0b10000, 0b11110, 0b10001, 0b01110},
	'7': {0b11111, 0b00001, 0b00010, 0b00100, 0b00100},
	'8': {0b01110, 0b10001, 0b01110, 0b10001, 0b01110},
	'9': {0b01110, 0b10001, 0b01111, 0b00001, 0b01110},

	' ':  {0b00000, 0b00000, 0b00000, 0b00000, 0b00000},
	'.':  {0b00000, 0b00000, 0b00000, 0b00000, 0b00100},
	',':  {0b00000, 0b00000, 0b00000, 0b00100, 0b01000},
	'!':  {0b00100, 0b00100, 0b00100, 0b00000, 0b00100},
	'?':  {0b01110, 0b10001, 0b00110, 0b00000, 0b00100},
	'\'': {0b00100, 0b00100, 0b00000, 0b00000, 0b00000},
	'"':  {0b01010, 0b01010, 0b00000, 0b00000, 0b00000},
	'-':  {0b00000, 0b00000, 0b11111, 0b00000, 0b00000},
	':':  {0b00000, 0b00100, 0b00000, 0b00100, 0b00000},
	';':  {0b00000, 0b00100, 0b00000, 0b00100, 0b01000},
	'(':  {0b00010, 0b00100, 0b00100, 0b00100, 0b00010},
	')':  {0b01000, 0b00100, 0b00100, 0b00100, 0b01000},
	'·':  {0b00000, 0b00000, 0b00100, 0b00000, 0b00000},

	// german letters
	'Ä': {0b01010, 0b01110, 0b10001, 0b11111, 0b10001},
	'Ö': {0b01010, 0b01110, 0b10001, 0b10001, 0b01110},
	'Ü': {0b01010, 0b10001, 0b10001, 0b10001, 0b01110},
	'ä': {0b01010, 0b01110, 0b00001, 0b01111, 0b01111},
	'ö': {0b01010, 0b00000, 0b01110, 0b10001, 0b01110},
	'ü': {0b01010, 0b00000, 0b10001, 0b10001, 0b01110},
	'ß': {0b01110, 0b10001, 0b11110, 0b10001, 0b11110},

	// polish letters
	'Ą': {0b01110, 0b10001, 0b11111, 0b10001, 0b10011},
	'Ć': {0b00010, 0b01111, 0b10000, 0b10000, 0b01111},
	'Ę': {0b11111, 0b10000, 0b11110, 0b10000, 0b11011},
	'Ł': {0b10000, 0b10000, 0b11100, 0b10000, 0b11111},
	'Ń': {0b00100, 0b10001, 0b11001, 0b10101, 0b10011},
	'Ó': {0b00100, 0b01110, 0b10001, 0b10001, 0b01110},
	'Ś': {0b00010, 0b01111, 0b10000, 0b01110, 0b11110},
	'Ź': {0b00010, 0b11111, 0b00010, 0b01000, 0b11111},
	'Ż': {0b00100, 0b11111, 0b00010, 0b01000, 0b11111},
	'ą': {0b00000, 0b01110, 0b00001, 0b01111, 0b01011},
	'ć': {0b00010, 0b01110, 0b10000, 0b10000, 0b01110},
	'ę': {0b01110, 0b10001, 0b11111, 0b10000, 0b01011},
	'ł': {0b01100, 0b00100, 0b01110, 0b00100, 0b01110},
	'ń': {0b00100, 0b00000, 0b11110, 0b10001, 0b10001},
	'ó': {0b00100, 0b00000, 0b01110, 0b10001, 0b01110},
	'ś': {0b00010, 0b00000, 0b01110, 0b11000, 0b01110},
	'ź': {0b00010, 0b00000, 0b11111, 0b00110, 0b11111},
	'ż': {0b00100, 0b00000, 0b11111, 0b00110, 0b11111},

	// french letters
	'À': {0b01000, 0b01110, 0b10001, 0b11111, 0b10001},
	'Â': {0b00100, 0b01110, 0b10001, 0b11111, 0b10001},
	'Ç': {0b01110, 0b10000, 0b10000, 0b01110, 0b00100},
	'È': {0b01000, 0b11111, 0b10000, 0b11110, 0b11111},
	'É': {0b00010, 0b11111, 0b10000, 0b11110, 0b11111},
	'Ê': {0b00100, 0b11111, 0b10000, 0b11110, 0b11111},
	'Ë': {0b01010, 0b11111, 0b10000, 0b11110, 0b11111},
	'Î': {0b00100, 0b01010, 0b00100, 0b00100, 0b11111},
	'Ï': {0b01010, 0b11111, 0b00100, 0b00100, 0b11111},
	'Ô': {0b00100, 0b01110, 0b10001, 0b10001, 0b01110},
	'Ù': {0b01000, 0b10001, 0b10001, 0b10001, 0b01110},
	'Û': {0b00100, 0b10001, 0b10001, 0b10001, 0b01110},
	'Ÿ': {0b01010, 0b10001, 0b01010, 0b00100, 0b00100},
	'Œ': {0b01111, 0b10101, 0b10111, 0b10101, 0b01111},
	'à': {0b01000, 0b01110, 0b00001, 0b01111, 0b01111},
	'â': {0b00100, 0b01110, 0b00001, 0b01111, 0b01111},
	'ç': {0b00000, 0b01110, 0b10000, 0b01110, 0b00100},
	'è': {0b01000, 0b01110, 0b11111, 0b10000, 0b01110},
	'é': {0b00010, 0b01110, 0b11111, 0b10000, 0b01110},
	'ê': {0b00100, 0b01110, 0b11111, 0b10000, 0b01110},
	'ë': {0b01010, 0b01110, 0b11111, 0b10000, 0b01110},
	'î': {0b00100, 0b01010, 0b00100, 0b00100, 0b00100},
	'ï': {0b01010, 0b00000, 0b00100, 0b00100, 0b00100},
	'ô': {0b00100, 0b00000, 0b01110, 0b10001, 0b01110},
	'ù': {0b01000, 0b00000, 0b10001, 0b10001, 0b01110},
	'û': {0b00100, 0b01010, 0b10001, 0b10001, 0b01110},
	'ÿ': {0b01010, 0b10001, 0b01111, 0b00001, 0b01110},
	'œ': {0b00000, 0b01111, 0b10101, 0b10100, 0b01111},
}

const (
	charWidth  = 5
	charHeight = 5
	charGap    = 1
)

type TextRenderer struct {
	palette     *artwork.Palette
	animState   *AnimState
	tickCount   int
	screenWidth int
}

func NewTextRenderer(palette *artwork.Palette, animState *AnimState, tickCount int, screenWidth int) *TextRenderer {
	return &TextRenderer{
		palette:     palette,
		animState:   animState,
		tickCount:   tickCount,
		screenWidth: screenWidth,
	}
}

func (r *TextRenderer) RenderFocusLyric(text string) []string {
	if text == "" {
		return nil
	}

	lines := r.wrapText(text)
	var result []string

	for _, line := range lines {
		runes := []rune(strings.ToUpper(line))
		totalPixelWidth := len(runes)*charWidth + (len(runes)-1)*charGap
		if totalPixelWidth < 0 {
			totalPixelWidth = 0
		}
		rendered := r.renderFocusText(runes, totalPixelWidth)
		result = append(result, rendered...)
	}

	return result
}

func (r *TextRenderer) RenderContextLyric(text string, brightness float64, isPast bool) []string {
	if text == "" {
		return nil
	}

	lines := r.wrapText(text)
	var result []string

	for _, line := range lines {
		runes := []rune(strings.ToUpper(line))
		rendered := r.renderContextText(runes, brightness, isPast)
		result = append(result, rendered...)
	}

	return result
}

func (r *TextRenderer) wrapText(text string) []string {
	maxPixelWidth := r.screenWidth - 8
	maxCharsPerLine := maxPixelWidth / (charWidth + charGap)
	if maxCharsPerLine < 5 {
		maxCharsPerLine = 5
	}

	words := strings.Fields(text)
	var lines []string
	var currentLine string

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if len(testLine) <= maxCharsPerLine {
			currentLine = testLine
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			if len(word) > maxCharsPerLine {
				currentLine = word[:maxCharsPerLine]
			} else {
				currentLine = word
			}
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

type pixelInfo struct {
	filled    bool
	charIndex int
	pixelX    int
}

func (r *TextRenderer) renderFocusText(runes []rune, totalPixelWidth int) []string {
	grid := make([][]pixelInfo, charHeight)
	for row := range grid {
		grid[row] = make([]pixelInfo, 0, totalPixelWidth)
	}

	charIndex := 0
	pixelX := 0

	for _, char := range runes {
		charData, ok := pixelFont[char]
		if !ok {
			charData = pixelFont[' ']
		}

		for row := 0; row < charHeight; row++ {
			for col := 0; col < charWidth; col++ {
				bit := (charData[row] >> (charWidth - 1 - col)) & 1
				grid[row] = append(grid[row], pixelInfo{
					filled:    bit == 1,
					charIndex: charIndex,
					pixelX:    pixelX + col,
				})
			}
		}

		pixelX += charWidth

		if charIndex < len(runes)-1 {
			for row := 0; row < charHeight; row++ {
				for g := 0; g < charGap; g++ {
					grid[row] = append(grid[row], pixelInfo{
						filled:    false,
						charIndex: charIndex,
						pixelX:    pixelX + g,
					})
				}
			}
			pixelX += charGap
		}

		charIndex++
	}

	return r.renderGridFocus(grid, len(runes), totalPixelWidth)
}

func (r *TextRenderer) renderContextText(runes []rune, brightness float64, isPast bool) []string {
	gridWidth := 0
	for i := range runes {
		gridWidth += charWidth
		if i < len(runes)-1 {
			gridWidth += charGap
		}
	}

	grid := make([][]pixelInfo, charHeight)
	for row := range grid {
		grid[row] = make([]pixelInfo, 0, gridWidth)
	}

	charIndex := 0
	pixelX := 0

	for _, char := range runes {
		charData, ok := pixelFont[char]
		if !ok {
			charData = pixelFont[' ']
		}

		for row := 0; row < charHeight; row++ {
			for col := 0; col < charWidth; col++ {
				bit := (charData[row] >> (charWidth - 1 - col)) & 1
				grid[row] = append(grid[row], pixelInfo{
					filled:    bit == 1,
					charIndex: charIndex,
					pixelX:    pixelX + col,
				})
			}
		}

		pixelX += charWidth

		if charIndex < len(runes)-1 {
			for row := 0; row < charHeight; row++ {
				for g := 0; g < charGap; g++ {
					grid[row] = append(grid[row], pixelInfo{
						filled:    false,
						charIndex: charIndex,
						pixelX:    pixelX + g,
					})
				}
			}
			pixelX += charGap
		}

		charIndex++
	}

	return r.renderGridContext(grid, gridWidth, brightness, isPast)
}

func (r *TextRenderer) renderGridFocus(grid [][]pixelInfo, totalChars int, totalPixelWidth int) []string {
	numTermRows := (charHeight + 1) / 2
	result := make([]string, numTermRows)

	centerPad := (r.screenWidth - totalPixelWidth) / 2
	if centerPad < 0 {
		centerPad = 0
	}
	padding := strings.Repeat(" ", centerPad)

	for termRow := 0; termRow < numTermRows; termRow++ {
		topRowIdx := termRow * 2
		bottomRowIdx := termRow*2 + 1

		var line strings.Builder
		line.WriteString(padding)

		for col := 0; col < totalPixelWidth && col < len(grid[0]); col++ {
			topPixel := grid[topRowIdx][col]
			var bottomPixel pixelInfo
			if bottomRowIdx < charHeight {
				bottomPixel = grid[bottomRowIdx][col]
			}

			topFilled := topPixel.filled
			bottomFilled := bottomRowIdx < charHeight && bottomPixel.filled

			color := r.calculateFocusColor(topPixel, topFilled || bottomFilled, totalChars, totalPixelWidth)
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

			if topFilled && bottomFilled {
				line.WriteString(style.Render("█"))
			} else if topFilled {
				line.WriteString(style.Render("▀"))
			} else if bottomFilled {
				line.WriteString(style.Render("▄"))
			} else {
				line.WriteString(" ")
			}
		}

		result[termRow] = line.String()
	}

	return result
}

func (r *TextRenderer) renderGridContext(grid [][]pixelInfo, totalPixelWidth int, brightness float64, isPast bool) []string {
	numTermRows := (charHeight + 1) / 2
	result := make([]string, numTermRows)

	centerPad := (r.screenWidth - totalPixelWidth) / 2
	if centerPad < 0 {
		centerPad = 0
	}
	padding := strings.Repeat(" ", centerPad)

	for termRow := 0; termRow < numTermRows; termRow++ {
		topRowIdx := termRow * 2
		bottomRowIdx := termRow*2 + 1

		var line strings.Builder
		line.WriteString(padding)

		for col := 0; col < totalPixelWidth && col < len(grid[0]); col++ {
			topPixel := grid[topRowIdx][col]
			var bottomPixel pixelInfo
			if bottomRowIdx < charHeight {
				bottomPixel = grid[bottomRowIdx][col]
			}

			topFilled := topPixel.filled
			bottomFilled := bottomRowIdx < charHeight && bottomPixel.filled

			color := r.calculateContextColor(topFilled || bottomFilled, brightness)
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

			if topFilled && bottomFilled {
				line.WriteString(style.Render("█"))
			} else if topFilled {
				line.WriteString(style.Render("▀"))
			} else if bottomFilled {
				line.WriteString(style.Render("▄"))
			} else {
				line.WriteString(" ")
			}
		}

		result[termRow] = line.String()
	}

	return result
}

func (r *TextRenderer) calculateFocusColor(pixel pixelInfo, anyFilled bool, totalChars int, totalPixelWidth int) string {
	if !anyFilled {
		return "#000000"
	}

	revealProgress := easeOutQuart(r.animState.CharReveal)
	waveSpeed := 0.03
	if totalChars > 20 {
		waveSpeed = 0.8 / float64(totalChars)
	}
	charRevealT := revealProgress - float64(pixel.charIndex)*waveSpeed
	if charRevealT < 0 {
		charRevealT = 0
	}
	if charRevealT > 1 {
		charRevealT = 1
	}

	if r.animState.CharReveal >= 1.0 {
		charRevealT = 1.0
	}

	gradientPos := 0.0
	if totalPixelWidth > 1 {
		gradientPos = float64(pixel.pixelX) / float64(totalPixelWidth-1)
	}

	baseColor := colors.BlendColors(r.palette.Primary, r.palette.Accent, gradientPos)

	if r.animState.GlowIntensity > 0.05 {
		baseColor = colors.AddGlow(baseColor, r.animState.GlowIntensity*0.5)
	}

	shimmer := math.Sin(r.animState.ShimmerPhase+float64(pixel.pixelX)*0.05)*0.5 + 0.5
	if shimmer > 0.5 {
		baseColor = colors.AddGlow(baseColor, (shimmer-0.5)*0.25)
	}

	rVal, gVal, bVal := colors.HexToRGB(baseColor)
	fadeT := easeOutCubic(charRevealT)
	rVal = int(float64(rVal) * fadeT)
	gVal = int(float64(gVal) * fadeT)
	bVal = int(float64(bVal) * fadeT)

	minBright := 15
	if rVal < minBright {
		rVal = minBright
	}
	if gVal < minBright {
		gVal = minBright
	}
	if bVal < minBright {
		bVal = minBright
	}

	return fmt.Sprintf("#%02X%02X%02X", rVal, gVal, bVal)
}

func (r *TextRenderer) calculateContextColor(anyFilled bool, brightness float64) string {
	if !anyFilled {
		return "#000000"
	}

	baseGrey := int(80 * brightness)
	if baseGrey < 25 {
		baseGrey = 25
	}
	if baseGrey > 80 {
		baseGrey = 80
	}

	return fmt.Sprintf("#%02X%02X%02X", baseGrey, baseGrey, baseGrey)
}
