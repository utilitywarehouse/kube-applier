package annotations

import "testing"

func TestBoolFloat64True(t *testing.T) {
	f := boolFloat64(true)

	if f != float64(1) {
		t.Errorf("Conversion of true was incorrect, got %g, want: %g", f, float64(1))
	}
}

func TestBoolFloat64False(t *testing.T) {
	f := boolFloat64(false)

	if f != float64(0) {
		t.Errorf("Conversion of false was incorrect, got %g, want: %g", f, float64(0))
	}
}
