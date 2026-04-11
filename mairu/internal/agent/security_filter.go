package agent

import (
	"fmt"
	"strings"
)

type SecurityFilter struct {
	BlockedCommands []string
	BlockedPaths    []string
}

func (s *SecurityFilter) Name() string {
	return "SecurityFilter"
}

func (s *SecurityFilter) PreExecute(ctx ToolContext, toolName string, args map[string]any) (map[string]any, error) {
	switch toolName {
	case "bash":
		if cmd, ok := args["command"].(string); ok {
			// First use the old IsDangerousCommand logic
			if IsDangerousCommand(cmd) {
				return args, &ErrRequiresApproval{Reason: fmt.Sprintf("Command matches built-in dangerous prefixes: %s", cmd)}
			}

			// Then check custom blocklist
			for _, blocked := range s.BlockedCommands {
				if strings.Contains(cmd, blocked) {
					return args, &ErrRequiresApproval{Reason: fmt.Sprintf("Command matches custom blocklist '%s': %s", blocked, cmd)}
				}
			}
		}
	case "write_file", "replace_block", "multi_edit", "delete_file":
		var path string
		if p, ok := args["file_path"].(string); ok {
			path = p
		} else if p, ok := args["path"].(string); ok {
			path = p
		}

		if path != "" {
			for _, blocked := range s.BlockedPaths {
				if strings.Contains(path, blocked) {
					return args, &ErrRequiresApproval{Reason: fmt.Sprintf("Path matches custom blocklist '%s': %s", blocked, path)}
				}
			}
		}
	}
	return args, nil
}
