package ai

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var kickoffLengthValue = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*(mm|cm|m|ft|米|厘米|毫米|英尺)?\s*$`)

// A unitless source number needs an explicit source-scoped human confirmation.
func kickoffLengthMeters(value, confirmedUnit string) string {
	parts := kickoffLengthValue.FindStringSubmatch(value)
	if parts == nil {
		return ""
	}
	unit := strings.ToLower(parts[2])
	if unit == "" {
		unit = confirmedUnit
	}
	scale, ok := map[string]float64{"m": 1, "米": 1, "cm": .01, "厘米": .01, "mm": .001, "毫米": .001, "ft": .3048, "英尺": .3048}[unit]
	if !ok {
		return ""
	}
	number, err := strconv.ParseFloat(parts[1], 64)
	number *= scale
	if err != nil || number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return ""
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}
