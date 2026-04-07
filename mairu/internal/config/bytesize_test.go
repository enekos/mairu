package config

import "testing"

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		err   bool
	}{
		{"512KB", 512 * 1024, false},
		{"1MB", 1024 * 1024, false},
		{"2GB", 2 * 1024 * 1024 * 1024, false},
		{"100B", 100, false},
		{"1024", 1024, false},
		{"0", 0, false},
		{"", 0, true},
		{"KB", 0, true},
		{"-1KB", 0, true},
		{"1.5MB", 0, true},
		{"1TB", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseByteSize(tt.input)
			if tt.err {
				if err == nil {
					t.Errorf("ParseByteSize(%q) = %d, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseByteSize(%q) error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseByteSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
