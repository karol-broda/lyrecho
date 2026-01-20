package artwork

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/charmbracelet/lipgloss"
	"github.com/nfnt/resize"

	"karolbroda.com/lyrecho/internal/colors"
)

type Palette struct {
	Primary      string
	Secondary    string
	Accent       string
	Dim          string
	Gradient     []string
	GradientInfo string // describes which color pair was selected for gradient
}

func Fetch(artworkURL string) (image.Image, error) {
	if artworkURL == "" {
		return nil, errors.New("empty artwork url")
	}

	if strings.HasPrefix(artworkURL, "file://") {
		path := strings.TrimPrefix(artworkURL, "file://")
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open artwork file: %w", err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("failed to decode artwork image: %w", err)
		}
		return img, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artworkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artwork: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artwork fetch returned status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode artwork: %w", err)
	}

	return img, nil
}

func ExtractPalette(img image.Image) *Palette {
	if img == nil {
		return DefaultPalette()
	}

	extractedColors, err := prominentcolor.KmeansWithAll(5, img, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, nil)
	if err != nil {
		return DefaultPalette()
	}

	if len(extractedColors) < 3 {
		return DefaultPalette()
	}

	type colorWithMetrics struct {
		color      prominentcolor.ColorItem
		sat        float64
		brightness float64
		score      float64
	}

	colorMetrics := make([]colorWithMetrics, len(extractedColors))
	for i, c := range extractedColors {
		r := float64(c.Color.R) / 255.0
		g := float64(c.Color.G) / 255.0
		b := float64(c.Color.B) / 255.0

		max := math.Max(math.Max(r, g), b)
		min := math.Min(math.Min(r, g), b)

		var sat float64
		if max == 0 {
			sat = 0
		} else {
			sat = (max - min) / max
		}

		brightness := max

		score := sat * (1.0 - math.Abs(brightness-0.6))

		colorMetrics[i] = colorWithMetrics{
			color:      c,
			sat:        sat,
			brightness: brightness,
			score:      score,
		}
	}

	var primary, secondary, accent colorWithMetrics
	maxScore := -1.0

	for _, cm := range colorMetrics {
		if cm.score > maxScore && cm.brightness > 0.3 && cm.sat > 0.2 {
			maxScore = cm.score
			primary = cm
		}
	}

	for _, cm := range colorMetrics {
		if cm.color.Color.R != primary.color.Color.R ||
			cm.color.Color.G != primary.color.Color.G ||
			cm.color.Color.B != primary.color.Color.B {
			if cm.sat > 0.15 && cm.brightness > 0.3 {
				secondary = cm
				break
			}
		}
	}

	for _, cm := range colorMetrics {
		if (cm.color.Color.R != primary.color.Color.R ||
			cm.color.Color.G != primary.color.Color.G ||
			cm.color.Color.B != primary.color.Color.B) &&
			(cm.color.Color.R != secondary.color.Color.R ||
				cm.color.Color.G != secondary.color.Color.G ||
				cm.color.Color.B != secondary.color.Color.B) {
			if cm.sat > 0.1 && cm.brightness > 0.25 {
				accent = cm
				break
			}
		}
	}

	primaryColor := boostColor(primary.color.Color.R, primary.color.Color.G, primary.color.Color.B, primary.brightness)
	secondaryColor := boostColor(secondary.color.Color.R, secondary.color.Color.G, secondary.color.Color.B, secondary.brightness)
	accentColor := boostColor(accent.color.Color.R, accent.color.Color.G, accent.color.Color.B, accent.brightness)

	selectedColors := []struct {
		color      string
		brightness float64
	}{
		{primaryColor, primary.brightness},
		{secondaryColor, secondary.brightness},
		{accentColor, accent.brightness},
	}

	for i := 0; i < len(selectedColors); i++ {
		for j := i + 1; j < len(selectedColors); j++ {
			if selectedColors[i].brightness < selectedColors[j].brightness {
				selectedColors[i], selectedColors[j] = selectedColors[j], selectedColors[i]
			}
		}
	}

	primaryColor = selectedColors[0].color
	secondaryColor = selectedColors[2].color
	accentColor = selectedColors[1].color

	// intelligently select the best color pair for the gradient
	gradStart, gradEnd, gradientInfo := selectBestGradientPair(primaryColor, secondaryColor, accentColor)

	return &Palette{
		Primary:      primaryColor,
		Secondary:    secondaryColor,
		Accent:       accentColor,
		Dim:          "#6272A4",
		Gradient:     colors.GenerateGradient(gradStart, gradEnd, 20),
		GradientInfo: gradientInfo,
	}
}

// selectBestGradientPair evaluates all possible color pairs and returns
// the pair that produces the smoothest gradient along with a description
func selectBestGradientPair(primary string, secondary string, accent string) (string, string, string) {
	// define all possible color pairs
	type colorPair struct {
		start      string
		end        string
		name       string
		smoothness float64
	}

	pairs := []colorPair{
		{primary, secondary, "primary → secondary", 0},
		{primary, accent, "primary → accent", 0},
		{secondary, primary, "secondary → primary", 0},
		{secondary, accent, "secondary → accent", 0},
		{accent, primary, "accent → primary", 0},
		{accent, secondary, "accent → secondary", 0},
	}

	// calculate smoothness for each pair
	steps := 20
	for i := range pairs {
		pairs[i].smoothness = colors.CalculateGradientSmoothness(pairs[i].start, pairs[i].end, steps)
	}

	// find the pair with lowest smoothness value (smoothest gradient)
	bestIdx := 0
	for i := 1; i < len(pairs); i++ {
		if pairs[i].smoothness < pairs[bestIdx].smoothness {
			bestIdx = i
		}
	}

	// prefer pairs that start with brighter colors for visual appeal
	// but only if the smoothness difference is very small (< 5)
	for i := range pairs {
		if i == bestIdx {
			continue
		}

		smoothnessDiff := pairs[i].smoothness - pairs[bestIdx].smoothness
		if smoothnessDiff < 5 {
			// check if this pair starts with a brighter color
			startL1 := colors.GetLightness(pairs[i].start)
			startL2 := colors.GetLightness(pairs[bestIdx].start)

			if startL1 > startL2 {
				bestIdx = i
			}
		}
	}

	return pairs[bestIdx].start, pairs[bestIdx].end, pairs[bestIdx].name
}

func DefaultPalette() *Palette {
	return &Palette{
		Primary:      "#8BA4E8",
		Secondary:    "#E8A4C8",
		Accent:       "#B8A8E8",
		Dim:          "#6272A4",
		Gradient:     colors.GenerateGradient("#8BA4E8", "#E8A4C8", 20),
		GradientInfo: "primary → secondary (default)",
	}
}

func boostColor(r, g, b uint32, brightness float64) string {
	if brightness < 0.4 {
		factor := 0.4 / brightness
		if factor > 2.5 {
			factor = 2.5
		}
		r = uint32(math.Min(255, float64(r)*factor))
		g = uint32(math.Min(255, float64(g)*factor))
		b = uint32(math.Min(255, float64(b)*factor))
	}

	if brightness > 0.85 {
		avg := (r + g + b) / 3
		factor := 0.7
		r = uint32(float64(avg) + (float64(r)-float64(avg))*factor)
		g = uint32(float64(avg) + (float64(g)-float64(avg))*factor)
		b = uint32(float64(avg) + (float64(b)-float64(avg))*factor)
	}

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func RenderHalfBlockArt(img image.Image, targetWidth int, targetHeight int) []string {
	if img == nil || targetWidth < 4 || targetHeight < 2 {
		return nil
	}

	actualHeight := targetHeight * 2

	resized := resize.Resize(uint(targetWidth), uint(actualHeight), img, resize.Lanczos3)
	bounds := resized.Bounds()

	lines := make([]string, targetHeight)

	for y := 0; y < targetHeight; y++ {
		var line strings.Builder
		topY := y * 2
		bottomY := topY + 1

		for x := 0; x < bounds.Dx(); x++ {
			topR, topG, topB, topA := resized.At(bounds.Min.X+x, bounds.Min.Y+topY).RGBA()

			var bottomR, bottomG, bottomB, bottomA uint32
			if bottomY < bounds.Dy() {
				bottomR, bottomG, bottomB, bottomA = resized.At(bounds.Min.X+x, bounds.Min.Y+bottomY).RGBA()
			} else {
				bottomR, bottomG, bottomB, bottomA = topR, topG, topB, topA
			}

			topR, topG, topB = topR>>8, topG>>8, topB>>8
			bottomR, bottomG, bottomB = bottomR>>8, bottomG>>8, bottomB>>8
			topA, bottomA = topA>>8, bottomA>>8

			if topA < 128 && bottomA < 128 {
				line.WriteString(" ")
				continue
			}

			topColor := fmt.Sprintf("#%02X%02X%02X", topR, topG, topB)
			bottomColor := fmt.Sprintf("#%02X%02X%02X", bottomR, bottomG, bottomB)

			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(topColor)).
				Background(lipgloss.Color(bottomColor))

			line.WriteString(style.Render("▀"))
		}
		lines[y] = line.String()
	}

	return lines
}
