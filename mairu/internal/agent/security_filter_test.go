package agent

import (
	"context"
	"strings"
	"testing"
)

func TestSecurityFilter(t *testing.T) {
	filter := &SecurityFilter{
		BlockedCommands: []string{"rm -rf /", "dd if="},
		BlockedPaths:    []string{".env", ".git/"},
	}

	tests := []struct {
		name        string
		toolName    string
		args        map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:     "Allowed bash command",
			toolName: "bash",
			args:     map[string]any{"command": "ls -la"},
			wantErr:  false,
		},
		{
			name:        "Built-in dangerous bash command",
			toolName:    "bash",
			args:        map[string]any{"command": "rm -rf ./temp"},
			wantErr:     true,
			errContains: "matches built-in dangerous prefixes",
		},
		{
			name:        "Custom blocked bash command",
			toolName:    "bash",
			args:        map[string]any{"command": "dd if=/dev/zero"},
			wantErr:     true,
			errContains: "matches custom blocklist",
		},
		{
			name:     "Allowed file write",
			toolName: "write_file",
			args:     map[string]any{"file_path": "src/main.go"},
			wantErr:  false,
		},
		{
			name:        "Blocked file write (.env)",
			toolName:    "write_file",
			args:        map[string]any{"file_path": "/var/www/.env"},
			wantErr:     true,
			errContains: "matches custom blocklist",
		},
		{
			name:        "Blocked file edit (.git/)",
			toolName:    "replace_block",
			args:        map[string]any{"file_path": ".git/config"},
			wantErr:     true,
			errContains: "matches custom blocklist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ToolContext{
				Context: context.Background(),
			}
			_, err := filter.PreExecute(ctx, tt.toolName, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("PreExecute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" {
				if _, ok := err.(*ErrRequiresApproval); !ok {
					t.Errorf("expected ErrRequiresApproval, got %T", err)
				}
				if err.Error() == "" || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}
