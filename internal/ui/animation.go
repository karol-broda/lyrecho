package ui

import (
	"math"
)

type AnimState struct {
	TransitionProgress float64
	CharReveal         float64
	GlowIntensity      float64
	ShimmerPhase       float64
	ScrollPosition     float64
	TargetScrollY      float64
	PrevScrollY        float64
}

func (a *AnimState) Reset() {
	a.TransitionProgress = 0
	a.CharReveal = 0
	a.GlowIntensity = 0
	a.ShimmerPhase = 0
	a.ScrollPosition = 0
	a.TargetScrollY = 0
	a.PrevScrollY = 0
}

func (a *AnimState) Update(tickCount int, newLine bool, transitionTicks int) {
	if transitionTicks <= 0 {
		transitionTicks = 18
	}

	if newLine {
		a.TransitionProgress = 0
		a.CharReveal = 0
		a.GlowIntensity = 1.0
		a.PrevScrollY = a.ScrollPosition
	}

	if a.TransitionProgress < 1.0 {
		speed := 1.0 / float64(transitionTicks)
		a.TransitionProgress += speed
		if a.TransitionProgress > 1.0 {
			a.TransitionProgress = 1.0
		}
	}

	if a.CharReveal < 1.0 {
		a.CharReveal += 0.08
		if a.CharReveal > 1.0 {
			a.CharReveal = 1.0
		}
	}

	scrollT := easeOutCubic(a.TransitionProgress)
	a.ScrollPosition = lerp(a.PrevScrollY, a.TargetScrollY, scrollT)

	if a.GlowIntensity > 0 {
		a.GlowIntensity *= 0.85
		if a.GlowIntensity < 0.01 {
			a.GlowIntensity = 0
		}
	}

	a.ShimmerPhase = float64(tickCount) * 0.05
}

func (a *AnimState) SlideOffset() float64 {
	return easeOutCubic(a.TransitionProgress)
}

func (a *AnimState) ScrollDelta() float64 {
	return a.ScrollPosition - a.PrevScrollY
}

func easeOutCubic(t float64) float64 {
	if t >= 1 {
		return 1
	}
	if t <= 0 {
		return 0
	}
	return 1 - math.Pow(1-t, 3)
}

func easeOutQuart(t float64) float64 {
	if t >= 1 {
		return 1
	}
	if t <= 0 {
		return 0
	}
	return 1 - math.Pow(1-t, 4)
}

func lerp(a float64, b float64, t float64) float64 {
	return a + (b-a)*t
}

func clamp(val float64, min float64, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
