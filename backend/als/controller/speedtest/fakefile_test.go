package speedtest

import (
	"strings"
	"testing"
)

func TestSizeToBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Valid sizes
		{name: "1 KB", input: "1KB", want: 1024},
		{name: "100 KB", input: "100KB", want: 102400},
		{name: "1 MB", input: "1MB", want: 1024 * 1024},
		{name: "10 MB", input: "10MB", want: 10 * 1024 * 1024},
		{name: "1 GB", input: "1GB", want: 1024 * 1024 * 1024},
		{name: "1 TB", input: "1TB", want: 1024 * 1024 * 1024 * 1024},

		// Zero is rejected: a 0-byte response would make downstream speed
		// measurement divide by zero and Content-Length: 0 is meaningless.
		{name: "0 KB rejected", input: "0KB", wantErr: true},

		// Invalid formats
		{name: "empty", input: "", wantErr: true},
		{name: "no unit", input: "1024", wantErr: true},
		{name: "unknown unit", input: "1XB", wantErr: true},
		{name: "lowercase unit not supported", input: "1mb", wantErr: true},
		{name: "mixed case unit", input: "1Mb", wantErr: true},
		{name: "trailing whitespace", input: "1MB ", wantErr: true},
		{name: "leading whitespace", input: " 1MB", wantErr: true},
		{name: "negative number rejected by regex", input: "-1MB", wantErr: true},
		{name: "plus sign rejected by regex", input: "+1MB", wantErr: true},
		{name: "decimal rejected by regex", input: "1.5MB", wantErr: true},
		{name: "trailing garbage", input: "1MBfoo", wantErr: true},
		{name: "leading garbage", input: "foo1MB", wantErr: true},
		{name: "double unit", input: "1MBGB", wantErr: true},

// Overflow: int64 max is ~9.2e18. 9007199254740992 = 2^63; multiplied by
// 2^40 it wraps to 0, which the new contract rejects. Inputs slightly
// above 2^63 wrap to a small positive value (integer modular wrap);
// out of scope for this fix -- the production allowlist
// (config.SpeedtestFileList) is the primary defense.
		{name: "TB wraps to zero rejected", input: "9007199254740992TB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sizeToBytes(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sizeToBytes(%q) = %d; want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sizeToBytes(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("sizeToBytes(%q) = %d; want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{name: "nil slice", slice: nil, item: "x", want: false},
		{name: "empty slice", slice: []string{}, item: "x", want: false},
		{name: "single hit", slice: []string{"1MB"}, item: "1MB", want: true},
		{name: "single miss", slice: []string{"1MB"}, item: "10MB", want: false},
		{name: "multi hit first", slice: []string{"1MB", "10MB", "100MB"}, item: "1MB", want: true},
		{name: "multi hit middle", slice: []string{"1MB", "10MB", "100MB"}, item: "10MB", want: true},
		{name: "multi hit last", slice: []string{"1MB", "10MB", "100MB"}, item: "100MB", want: true},
		{name: "multi miss", slice: []string{"1MB", "10MB", "100MB"}, item: "1GB", want: false},
		{name: "case sensitive", slice: []string{"1MB"}, item: "1mb", want: false},
		{name: "empty string item", slice: []string{""}, item: "", want: true},
		{name: "empty string item miss", slice: []string{"1MB"}, item: "", want: false},
		{name: "whitespace item miss", slice: []string{"1MB"}, item: " 1MB", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := contains(tt.slice, tt.item); got != tt.want {
				t.Errorf("contains(%v, %q) = %v; want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func FuzzSizeToBytes(f *testing.F) {
	// Seed corpus: representative valid and adversarial inputs.
	seeds := []string{
		"1KB", "1MB", "1GB", "1TB",
		"100MB", "0KB",
		"", "1", "1XB", "1mb",
		"-1MB", "+1MB", "1.5MB", "1MBfoo", "foo1MB",
		strings.Repeat("9", 50) + "MB",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got, err := sizeToBytes(input)
		if err != nil {
			// On error, the byte count must be zero (no partial work).
			if got != 0 {
				t.Errorf("sizeToBytes(%q) = (%d, %v); want zero on error", input, got, err)
			}
			return
		}
		// On success, the byte count must be strictly positive.
		if got <= 0 {
			t.Errorf("sizeToBytes(%q) = %d; want > 0 on success", input, got)
		}
	})
}