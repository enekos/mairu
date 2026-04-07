package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseByteSize parses a human-readable byte size string like "512KB", "1MB", "2GB"
// into its int64 byte value. Plain integers (no suffix) are treated as bytes.
// Supported suffixes: B, KB, MB, GB (case-insensitive).
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty byte size")
	}

	upper := strings.ToUpper(s)
	suffixes := []struct {
		suffix     string
		multiplier int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(upper, sf.suffix) {
			numStr := strings.TrimSuffix(upper, sf.suffix)
			if numStr == "" {
				return 0, fmt.Errorf("missing number in byte size %q", s)
			}
			n, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
			}
			if n < 0 {
				return 0, fmt.Errorf("negative byte size %q", s)
			}
			return n * sf.multiplier, nil
		}
	}

	// No suffix — treat as raw bytes.
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
	}
	if n < 0 {
		return 0, fmt.Errorf("negative byte size %q", s)
	}
	return n, nil
}
