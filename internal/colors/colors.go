package colors

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func GenerateGradient(startHex string, endHex string, steps int) []string {
	if steps < 2 {
		steps = 2
	}

	sr, sg, sb := HexToRGB(startHex)
	er, eg, eb := HexToRGB(endHex)

	// convert to lch for perceptually uniform color interpolation
	sl, sc, sh := rgbToLCH(sr, sg, sb)
	el, ec, eh := rgbToLCH(er, eg, eb)

	// find shortest hue path (avoid going around the color wheel the long way)
	hueDiff := eh - sh
	if hueDiff > 180 {
		hueDiff -= 360
	} else if hueDiff < -180 {
		hueDiff += 360
	}

	// calculate color differences for adaptive smoothing
	chromaDiff := math.Abs(ec - sc)
	lightnessDiff := math.Abs(el - sl)
	hueDistance := math.Abs(hueDiff)

	// determine if we need smoothing based on color distance
	needsSmoothing := chromaDiff > 30 || hueDistance > 60 || lightnessDiff > 30

	gradient := make([]string, steps)
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)

		// apply adaptive smoothing when colors are very different
		tInterp := t
		if needsSmoothing {
			// use double smoothstep for more gradual transitions at extremes
			tInterp = smoothStep(smoothStep(t))
		}

		// interpolate in lch space with smoothed t
		l := sl + tInterp*(el-sl)
		c := sc + tInterp*(ec-sc)
		h := sh + tInterp*hueDiff
		if h < 0 {
			h += 360
		} else if h >= 360 {
			h -= 360
		}

		// convert back to rgb
		r, g, b := lchToRGB(l, c, h)
		gradient[i] = fmt.Sprintf("#%02X%02X%02X", r, g, b)
	}

	return gradient
}

func GenerateMultiGradient(colors []string, steps int) []string {
	if len(colors) < 2 || steps < 2 {
		if len(colors) >= 1 {
			return []string{colors[0]}
		}
		return []string{"#FFFFFF"}
	}

	gradient := make([]string, 0, steps)
	segments := len(colors) - 1
	stepsPerSegment := steps / segments

	for i := 0; i < segments; i++ {
		segSteps := stepsPerSegment
		if i == segments-1 {
			segSteps = steps - len(gradient)
		}
		segGrad := GenerateGradient(colors[i], colors[i+1], segSteps+1)
		if i == 0 {
			gradient = append(gradient, segGrad...)
		} else {
			gradient = append(gradient, segGrad[1:]...)
		}
	}

	return gradient
}

// CalculateGradientSmoothness evaluates how smooth a gradient between two colors would be.
// returns the maximum perceptual color jump that would occur in the gradient.
// lower values indicate smoother gradients (ideal: < 35, acceptable: < 50, problematic: > 50)
func CalculateGradientSmoothness(startHex string, endHex string, steps int) float64 {
	if steps < 2 {
		steps = 2
	}

	gradient := GenerateGradient(startHex, endHex, steps)
	maxJump := 0.0

	for i := 1; i < len(gradient); i++ {
		r1, g1, b1 := HexToRGB(gradient[i-1])
		r2, g2, b2 := HexToRGB(gradient[i])

		// calculate perceptual distance using redmean approximation
		rmean := (r1 + r2) / 2
		dr := r1 - r2
		dg := g1 - g2
		db := b1 - b2

		distance := math.Sqrt(
			float64((2+rmean/256)*dr*dr + 4*dg*dg + (2+(255-rmean)/256)*db*db),
		)

		if distance > maxJump {
			maxJump = distance
		}
	}

	return maxJump
}

// GetLightness returns the perceived lightness of a color (0-100 scale)
// uses the L component from LCH color space for perceptual accuracy
func GetLightness(hexColor string) float64 {
	r, g, b := HexToRGB(hexColor)
	l, _, _ := rgbToLCH(r, g, b)
	return l
}

func BlendColors(hex1 string, hex2 string, t float64) string {
	r1, g1, b1 := HexToRGB(hex1)
	r2, g2, b2 := HexToRGB(hex2)

	// convert to lch for perceptually uniform blending
	l1, c1, h1 := rgbToLCH(r1, g1, b1)
	l2, c2, h2 := rgbToLCH(r2, g2, b2)

	// find shortest hue path
	hueDiff := h2 - h1
	if hueDiff > 180 {
		hueDiff -= 360
	} else if hueDiff < -180 {
		hueDiff += 360
	}

	// interpolate in lch space
	l := l1 + t*(l2-l1)
	c := c1 + t*(c2-c1)
	h := h1 + t*hueDiff
	if h < 0 {
		h += 360
	} else if h >= 360 {
		h -= 360
	}

	// convert back to rgb
	r, g, b := lchToRGB(l, c, h)
	return RGBToHex(r, g, b)
}

func RGBToHex(r int, g int, b int) string {
	r = clampInt(r, 0, 255)
	g = clampInt(g, 0, 255)
	b = clampInt(b, 0, 255)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func clampInt(val int, min int, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func AdjustBrightness(hex string, factor float64) string {
	r, g, b := HexToRGB(hex)
	r = clampInt(int(float64(r)*factor), 0, 255)
	g = clampInt(int(float64(g)*factor), 0, 255)
	b = clampInt(int(float64(b)*factor), 0, 255)
	return RGBToHex(r, g, b)
}

func AddGlow(hex string, intensity float64) string {
	r, g, b := HexToRGB(hex)
	boost := 1.0 + intensity*0.6
	r = clampInt(int(float64(r)*boost), 0, 255)
	g = clampInt(int(float64(g)*boost), 0, 255)
	b = clampInt(int(float64(b)*boost), 0, 255)
	return RGBToHex(r, g, b)
}

func Desaturate(hex string, amount float64) string {
	r, g, b := HexToRGB(hex)
	gray := int(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b))
	r = clampInt(int(float64(r)+(float64(gray)-float64(r))*amount), 0, 255)
	g = clampInt(int(float64(g)+(float64(gray)-float64(g))*amount), 0, 255)
	b = clampInt(int(float64(b)+(float64(gray)-float64(b))*amount), 0, 255)
	return RGBToHex(r, g, b)
}

func ShiftHue(hex string, shift float64) string {
	r, g, b := HexToRGB(hex)
	h, s, l := rgbToHSL(r, g, b)
	h = math.Mod(h+shift, 360)
	if h < 0 {
		h += 360
	}
	nr, ng, nb := hslToRGB(h, s, l)
	return RGBToHex(nr, ng, nb)
}

// smoothStep applies ease-in-ease-out smoothing using smoothstep function
func smoothStep(t float64) float64 {
	// clamp t to 0-1 range
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	// smoothstep formula: 3t^2 - 2t^3
	return t * t * (3 - 2*t)
}

func rgbToLCH(r int, g int, b int) (float64, float64, float64) {
	// convert rgb to xyz
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	// apply gamma correction
	if rf > 0.04045 {
		rf = math.Pow((rf+0.055)/1.055, 2.4)
	} else {
		rf = rf / 12.92
	}
	if gf > 0.04045 {
		gf = math.Pow((gf+0.055)/1.055, 2.4)
	} else {
		gf = gf / 12.92
	}
	if bf > 0.04045 {
		bf = math.Pow((bf+0.055)/1.055, 2.4)
	} else {
		bf = bf / 12.92
	}

	// convert to xyz (d65 illuminant)
	x := rf*0.4124564 + gf*0.3575761 + bf*0.1804375
	y := rf*0.2126729 + gf*0.7151522 + bf*0.0721750
	z := rf*0.0193339 + gf*0.1191920 + bf*0.9503041

	// convert xyz to lab
	x = x / 0.95047
	y = y / 1.00000
	z = z / 1.08883

	labFunc := func(t float64) float64 {
		if t > 0.008856 {
			return math.Pow(t, 1.0/3.0)
		}
		return (7.787 * t) + (16.0 / 116.0)
	}

	x = labFunc(x)
	y = labFunc(y)
	z = labFunc(z)

	l := (116.0 * y) - 16.0
	labA := 500.0 * (x - y)
	labB := 200.0 * (y - z)

	// convert lab to lch
	c := math.Sqrt(labA*labA + labB*labB)
	h := math.Atan2(labB, labA) * 180.0 / math.Pi
	if h < 0 {
		h += 360
	}

	return l, c, h
}

func lchToRGB(l float64, c float64, h float64) (int, int, int) {
	// convert lch to lab
	hRad := h * math.Pi / 180.0
	labA := c * math.Cos(hRad)
	labB := c * math.Sin(hRad)

	// convert lab to xyz
	y := (l + 16.0) / 116.0
	x := labA/500.0 + y
	z := y - labB/200.0

	labInvFunc := func(t float64) float64 {
		t3 := t * t * t
		if t3 > 0.008856 {
			return t3
		}
		return (t - 16.0/116.0) / 7.787
	}

	x = labInvFunc(x) * 0.95047
	y = labInvFunc(y) * 1.00000
	z = labInvFunc(z) * 1.08883

	// convert xyz to rgb
	rLin := x*3.2404542 + y*-1.5371385 + z*-0.4985314
	gLin := x*-0.9692660 + y*1.8760108 + z*0.0415560
	bLin := x*0.0556434 + y*-0.2040259 + z*1.0572252

	// apply inverse gamma correction
	gammaInv := func(t float64) float64 {
		if t > 0.0031308 {
			return 1.055*math.Pow(t, 1.0/2.4) - 0.055
		}
		return 12.92 * t
	}

	rLin = gammaInv(rLin)
	gLin = gammaInv(gLin)
	bLin = gammaInv(bLin)

	// convert to 0-255 range
	ri := clampInt(int(rLin*255.0+0.5), 0, 255)
	gi := clampInt(int(gLin*255.0+0.5), 0, 255)
	bi := clampInt(int(bLin*255.0+0.5), 0, 255)

	return ri, gi, bi
}

func rgbToHSL(r int, g int, b int) (float64, float64, float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	max := math.Max(math.Max(rf, gf), bf)
	min := math.Min(math.Min(rf, gf), bf)
	l := (max + min) / 2

	if max == min {
		return 0, 0, l
	}

	d := max - min
	var s float64
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}

	var h float64
	switch max {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h *= 60

	return h, s, l
}

func hslToRGB(h float64, s float64, l float64) (int, int, int) {
	if s == 0 {
		v := int(l * 255)
		return v, v, v
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	hk := h / 360

	tr := hk + 1.0/3.0
	tg := hk
	tb := hk - 1.0/3.0

	adjustT := func(t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		return t
	}

	tr = adjustT(tr)
	tg = adjustT(tg)
	tb = adjustT(tb)

	calcColor := func(t float64) float64 {
		if t < 1.0/6.0 {
			return p + (q-p)*6*t
		}
		if t < 0.5 {
			return q
		}
		if t < 2.0/3.0 {
			return p + (q-p)*(2.0/3.0-t)*6
		}
		return p
	}

	r := int(calcColor(tr) * 255)
	g := int(calcColor(tg) * 255)
	b := int(calcColor(tb) * 255)

	return clampInt(r, 0, 255), clampInt(g, 0, 255), clampInt(b, 0, 255)
}

func HexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 255, 255, 255
	}

	r, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		r = 255
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		g = 255
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		b = 255
	}

	return int(r), int(g), int(b)
}

func RenderGradientText(text string, gradient []string, bold bool) string {
	if len(text) == 0 {
		return ""
	}
	if len(gradient) == 0 {
		return text
	}

	runes := []rune(text)
	var result strings.Builder

	for i, r := range runes {
		colorIdx := 0
		if len(runes) > 1 {
			colorIdx = i * (len(gradient) - 1) / (len(runes) - 1)
		}
		if colorIdx >= len(gradient) {
			colorIdx = len(gradient) - 1
		}

		style := lipgloss.NewStyle().Foreground(lipgloss.Color(gradient[colorIdx]))
		if bold {
			style = style.Bold(true)
		}
		result.WriteString(style.Render(string(r)))
	}

	return result.String()
}

func FormatTime(seconds int64) string {
	if seconds < 0 {
		return "0:00"
	}
	minutes := seconds / 60
	remaining := seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, remaining)
}
