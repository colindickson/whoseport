package terminal

import "testing"

func TestClampWidth(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"within range unchanged", 80, 80},
		{"above max clamped down", 200, MaxWidth},
		{"below min clamped up", 10, MinWidth},
		{"zero clamped up", 0, MinWidth},
		{"exactly max", MaxWidth, MaxWidth},
		{"exactly min", MinWidth, MinWidth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampWidth(tt.in); got != tt.want {
				t.Errorf("clampWidth(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
