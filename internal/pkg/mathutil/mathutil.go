package mathutil

import "math"

func RoundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
