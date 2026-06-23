package iperf3

import (
	"testing"
)

func TestRandomPortInRange(t *testing.T) {
	tests := []struct {
		name string
		min  int
		max  int
	}{
		{"normal range", 30000, 31000},
		{"narrow range", 5000, 5001},
		{"single port", 8080, 8080},
		{"large range", 1, 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run several iterations since randomPort uses crypto/rand.
			for i := 0; i < 100; i++ {
				port, err := randomPort(tt.min, tt.max)
				if err != nil {
					t.Fatalf("randomPort(%d, %d) error: %v", tt.min, tt.max, err)
				}
				if port < tt.min || port > tt.max {
					t.Errorf("randomPort(%d, %d) = %d; out of range", tt.min, tt.max, port)
				}
			}
		})
	}
}

func TestRandomPortInvalidRange(t *testing.T) {
	tests := []struct {
		name string
		min  int
		max  int
	}{
		{"max less than min", 100, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := randomPort(tt.min, tt.max)
			if err == nil {
				t.Errorf("randomPort(%d, %d) = %d; want error", tt.min, tt.max, port)
			}
		})
	}
}

// randomPort does not validate that the values are positive port
// numbers -- only that max >= min. Negative values are accepted but
// cannot be opened. We document the current behaviour here.
func TestRandomPortAcceptsNegativeRange(t *testing.T) {
	port, err := randomPort(-10, -1)
	if err != nil {
		t.Errorf("randomPort(-10, -1) error: %v", err)
	}
	if port < -10 || port > -1 {
		t.Errorf("randomPort(-10, -1) = %d; out of range", port)
	}
}