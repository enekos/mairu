package tui

import "testing"

func TestAllSlashCommandsContainsCouncilCommand(t *testing.T) {
	for _, cmd := range allSlashCommands {
		if cmd.Name == "/council" {
			return
		}
	}
	t.Fatalf("expected /council slash command to be registered")
}
