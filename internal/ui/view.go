package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"karolbroda.com/lyrecho/internal/artwork"
	"karolbroda.com/lyrecho/internal/colors"
	"karolbroda.com/lyrecho/internal/terminal"
)

func (m Model) View() string {
	width := m.width
	height := m.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	if m.quitting {
		return ""
	}

	palette := m.display.Palette
	if palette == nil {
		palette = artwork.DefaultPalette()
	}

	if m.display.Track == nil {
		return m.renderWaitingScreen(palette, width, height)
	}

	return m.renderMainScreen(palette, width, height)
}

func (m Model) renderWaitingScreen(palette *artwork.Palette, width int, height int) string {
	var lines []string

	for y := 0; y < height; y++ {
		centerY := height / 2

		if y == centerY-1 {
			waitText := "awaiting music"
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(palette.Dim)).
				Italic(true)
			centered := centerText(style.Render(waitText), len(waitText), width)
			lines = append(lines, centered)
		} else if y == centerY {
			pulseChars := []string{"·", "•", "●", "•"}
			pulseIdx := (m.tickCount / 4) % len(pulseChars)
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Secondary))
			lines = append(lines, centerText(style.Render(pulseChars[pulseIdx]), 1, width))
		} else {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderMainScreen(palette *artwork.Palette, width int, height int) string {
	var lines []string

	headerHeight := 0

	if !m.hideHeader {
		headerLines := m.renderCompactHeader(palette, width)
		lines = append(lines, headerLines...)
		headerHeight = len(headerLines)
	}

	lyricsHeight := height - headerHeight

	if m.err != nil {
		lines = append(lines, m.renderErrorSection(palette, lyricsHeight, width)...)
	} else if m.display.CurrentIndex >= 0 && m.display.CurrentIndex < len(m.display.Lines) {
		lines = append(lines, m.renderSlidingLyrics(palette, lyricsHeight, width)...)
	} else {
		lines = append(lines, m.renderWaitingForLyrics(palette, lyricsHeight, width)...)
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderCompactHeader(palette *artwork.Palette, width int) []string {
	var lines []string

	lines = append(lines, "")

	artWidth := 12
	artHeight := 6
	if width < 80 {
		artWidth = 8
		artHeight = 4
	}
	if width < 50 || m.height < 25 {
		artWidth = 0
		artHeight = 0
	}

	var artworkLines []string
	useKittyGraphics := m.termCaps != nil && m.termCaps.SupportsKittyGraphics && artWidth > 0 && m.display.Image != nil

	if useKittyGraphics {
		// use kitty graphics protocol
		kittyImageOutput := terminal.EncodeImageForKitty(m.display.Image, artWidth, artHeight)
		if kittyImageOutput == "" {
			// fallback to half-block rendering if encoding fails
			useKittyGraphics = false
			artworkLines = artwork.RenderHalfBlockArt(m.display.Image, artWidth, artHeight)
		} else {
			// output kitty image with proper indentation
			lines = append(lines, "  "+kittyImageOutput)
			// add placeholder lines with proper indentation for vertical spacing where image occupies space
			for i := 0; i < artHeight-1; i++ {
				lines = append(lines, "  ")
			}
		}
	} else {
		artworkLines = artwork.RenderHalfBlockArt(m.display.Image, artWidth, artHeight)
	}

	infoLines := m.renderTrackInfo(palette, width)

	if useKittyGraphics {
		// with kitty graphics, add info lines below the image with indent
		for _, infoLine := range infoLines {
			lines = append(lines, "  "+infoLine)
		}
		lines = append(lines, "")
	} else {
		// with half-block art, display side-by-side
		maxLines := artHeight
		if len(infoLines) > maxLines {
			maxLines = len(infoLines)
		}
		if artHeight == 0 {
			maxLines = len(infoLines)
		}

		for i := 0; i < maxLines; i++ {
			var line strings.Builder

			if artWidth > 0 && i < len(artworkLines) {
				line.WriteString("  ")
				line.WriteString(artworkLines[i])
				line.WriteString("  ")
			} else if artWidth > 0 {
				line.WriteString(strings.Repeat(" ", artWidth+4))
			}

			if i < len(infoLines) {
				line.WriteString(infoLines[i])
			}

			lines = append(lines, line.String())
		}
	}

	lines = append(lines, "")

	trk := m.display.Track
	if trk != nil && trk.DurationSecs > 0 {
		progressBar := m.renderMinimalProgress(palette, width)
		lines = append(lines, progressBar)
	}

	lines = append(lines, "")

	return lines
}

func (m Model) renderTrackInfo(palette *artwork.Palette, width int) []string {
	trk := m.display.Track
	if trk == nil {
		return nil
	}

	var lines []string

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(palette.Primary)).
		Bold(true)

	artistStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(palette.Secondary))

	maxWidth := width - 20
	if maxWidth < 20 {
		maxWidth = 20
	}

	title := trk.Title
	if len(title) > maxWidth {
		title = title[:maxWidth-1] + "…"
	}
	lines = append(lines, titleStyle.Render(title))

	artist := trk.Artist
	if len(artist) > maxWidth {
		artist = artist[:maxWidth-1] + "…"
	}
	lines = append(lines, artistStyle.Render(artist))

	if trk.Album != "" {
		albumStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Dim))
		album := trk.Album
		if len(album) > maxWidth {
			album = album[:maxWidth-1] + "…"
		}
		lines = append(lines, albumStyle.Render(album))
	}

	return lines
}

func (m Model) renderMinimalProgress(palette *artwork.Palette, width int) string {
	trk := m.display.Track
	if trk == nil || trk.DurationSecs == 0 {
		return ""
	}

	barWidth := width - 20
	if barWidth < 20 {
		barWidth = 20
	}

	progress := float64(m.positionSecs) / float64(trk.DurationSecs)
	if progress > 1.0 {
		progress = 1.0
	}
	if progress < 0 {
		progress = 0
	}

	filledWidth := int(float64(barWidth) * progress)

	var bar strings.Builder

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Primary))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Dim)).Faint(true)

	for i := 0; i < barWidth; i++ {
		if i < filledWidth {
			bar.WriteString(filledStyle.Render("━"))
		} else if i == filledWidth {
			bar.WriteString(filledStyle.Render("●"))
		} else {
			bar.WriteString(emptyStyle.Render("─"))
		}
	}

	currentTime := colors.FormatTime(m.positionSecs)
	totalTime := colors.FormatTime(trk.DurationSecs)

	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Dim))

	return fmt.Sprintf("  %s  %s  %s",
		timeStyle.Render(currentTime),
		bar.String(),
		timeStyle.Render(totalTime))
}

func (m Model) renderSlidingLyrics(palette *artwork.Palette, height int, width int) []string {
	renderer := NewTextRenderer(palette, &m.animState, m.tickCount, width)

	slideT := m.animState.SlideOffset()

	output := make([]string, height)
	for i := range output {
		output[i] = ""
	}

	contextCount := 2
	if height < 20 {
		contextCount = 1
	}

	type renderedLyric struct {
		lines      []string
		offset     int
		isFocus    bool
		brightness float64
	}

	var allLyrics []renderedLyric

	for offset := -contextCount - 1; offset <= contextCount+1; offset++ {
		idx := m.display.CurrentIndex + offset
		if idx < 0 || idx >= len(m.display.Lines) {
			continue
		}

		text := m.display.Lines[idx].Text
		if text == "" {
			text = "···"
		}

		var brightness float64
		var isFocus bool

		if offset == 0 {
			brightness = 1.0
			isFocus = true
		} else if offset == -1 && slideT < 1.0 {
			brightness = lerp(0.7, 0.4, slideT)
			isFocus = false
		} else if offset == 1 && slideT < 1.0 {
			brightness = lerp(0.35, 0.5, slideT)
			isFocus = false
		} else {
			dist := offset
			if dist < 0 {
				dist = -dist
			}
			brightness = 0.5 - float64(dist-1)*0.1
			if brightness < 0.3 {
				brightness = 0.3
			}
			isFocus = false
		}

		var rendered []string
		if isFocus {
			rendered = renderer.RenderFocusLyric(text)
		} else {
			isPast := offset < 0
			rendered = renderer.RenderContextLyric(text, brightness, isPast)
		}

		allLyrics = append(allLyrics, renderedLyric{
			lines:      rendered,
			offset:     offset,
			isFocus:    isFocus,
			brightness: brightness,
		})
	}

	var currentLyricIdx int
	var currentLyricHeight int
	for i, rl := range allLyrics {
		if rl.offset == 0 {
			currentLyricIdx = i
			currentLyricHeight = len(rl.lines)
			break
		}
	}

	centerY := (height - currentLyricHeight) / 2
	if centerY < 0 {
		centerY = 0
	}

	spacing := 2
	slideAmount := float64(currentLyricHeight + spacing)

	positions := make([]int, len(allLyrics))
	positions[currentLyricIdx] = centerY

	y := centerY
	for i := currentLyricIdx - 1; i >= 0; i-- {
		y -= len(allLyrics[i].lines) + spacing
		positions[i] = y
	}

	y = centerY + currentLyricHeight + spacing
	for i := currentLyricIdx + 1; i < len(allLyrics); i++ {
		positions[i] = y
		y += len(allLyrics[i].lines) + spacing
	}

	slideOffset := int(slideT * slideAmount)

	for pass := 0; pass < 2; pass++ {
		for i, rl := range allLyrics {
			if pass == 0 && rl.isFocus {
				continue
			}
			if pass == 1 && !rl.isFocus {
				continue
			}

			finalY := positions[i] - slideOffset

			for j, line := range rl.lines {
				row := finalY + j
				if row >= 0 && row < height {
					if output[row] == "" || rl.isFocus {
						output[row] = line
					}
				}
			}
		}
	}

	return output
}

func (m Model) renderErrorSection(palette *artwork.Palette, height int, width int) []string {
	lines := make([]string, 0, height)

	for i := 0; i < height/2-1; i++ {
		lines = append(lines, "")
	}

	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B"))

	errText := m.err.Error()
	lines = append(lines, centerText(errStyle.Render(errText), len(errText), width))

	return lines
}

func (m Model) renderWaitingForLyrics(palette *artwork.Palette, height int, width int) []string {
	lines := make([]string, 0, height)

	for i := 0; i < height/2-1; i++ {
		lines = append(lines, "")
	}

	if m.loadingState.IsLoadingLyrics() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		idx := m.tickCount % len(frames)
		spinnerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Secondary))
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Dim))
		msgText := spinnerStyle.Render(frames[idx]) + textStyle.Render(" loading")
		lines = append(lines, centerText(msgText, 10, width))
	} else if m.display.CurrentIndex >= len(m.display.Lines) {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Dim))
		lines = append(lines, centerText(style.Render("·"), 1, width))
	} else {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(palette.Dim))
		lines = append(lines, centerText(style.Render("♪"), 1, width))
	}

	return lines
}

func centerText(text string, visualWidth int, screenWidth int) string {
	padding := (screenWidth - visualWidth) / 2
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat(" ", padding) + text
}
