package verdictcheck

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// canonicalNumbers retains arbitrary-size JSON integers and matches Python's
// JSON float representation used by the independent reference digest command.
func canonicalNumbers(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			n, err := canonicalNumbers(item)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			n, err := canonicalNumbers(item)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	case json.Number:
		s := string(v)
		if !strings.ContainsAny(s, ".eE") {
			if s == "-0" {
				return json.Number("0"), nil
			}
			return v, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		return canonicalFloat(f)
	case float64:
		return canonicalFloat(v)
	case float32:
		return canonicalFloat(float64(v))
	default:
		return value, nil
	}
}
func canonicalFloat(f float64) (json.Number, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", fmt.Errorf("nonfinite JSON number")
	}
	format := byte('f')
	abs := math.Abs(f)
	if abs != 0 && (abs < 1e-4 || abs >= 1e16) {
		format = 'e'
	}
	s := strconv.FormatFloat(f, format, -1, 64)
	if format == 'f' && !strings.Contains(s, ".") {
		s += ".0"
	}
	return json.Number(s), nil
}
