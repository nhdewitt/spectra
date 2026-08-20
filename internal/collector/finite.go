package collector

import (
	"math"
	"reflect"
)

// maxFiniteDepth bounds the walk below. Metric structs nest three levels at
// most (list metric -> element struct -> pointer field), so this is a guard
// against a future cyclic type, not a real limit.
const maxFiniteDepth = 8

// hasNonFinite reports whether v holds a NaN or an infinity anywhere in its
// exported fields.
//
// encoding/json refuses to marshal either one, and a metric batch is encoded as
// a unit, so a single bad float fails an entire batch, or an entire cache drain,
// which can be thousands of envelopes.
//
// The reflective walk covers every metric type without each one having to
// implement a check, including types added later. Unexported fields are skipped.
func hasNonFinite(v any) bool {
	return nonFinite(reflect.ValueOf(v), 0)
}

func nonFinite(v reflect.Value, depth int) bool {
	if depth > maxFiniteDepth || !v.IsValid() {
		return false
	}

	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		return math.IsNaN(f) || math.IsInf(f, 0)

	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return false
		}
		return nonFinite(v.Elem(), depth+1)

	case reflect.Slice, reflect.Array:
		// []float64 is the common shape (CPUMetric.CoreUsage) - read it
		// directly rather than reflecting per element.
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Float64 && v.CanInterface() {
			for _, f := range v.Interface().([]float64) {
				if math.IsNaN(f) || math.IsInf(f, 0) {
					return true
				}
			}
			return false
		}
		for i := range v.Len() {
			if nonFinite(v.Index(i), depth+1) {
				return true
			}
		}
		return false

	case reflect.Struct:
		t := v.Type()
		for i := range v.NumField() {
			if !t.Field(i).IsExported() {
				continue
			}
			if nonFinite(v.Field(i), depth+1) {
				return true
			}
		}
		return false

	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if nonFinite(iter.Value(), depth+1) {
				return true
			}
		}
		return false

	default:
		return false
	}
}
