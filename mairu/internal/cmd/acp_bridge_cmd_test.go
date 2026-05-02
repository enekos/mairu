//go:build !slim && !headless && !contextsrvonly

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestACPBridgeCmd_HasFlags(t *testing.T) {
	c := NewACPBridgeCmd()
	if f := c.Flags().Lookup("addr"); f == nil {
		t.Fatal("--addr flag missing")
	} else if f.DefValue != "127.0.0.1:7777" {
		t.Errorf("--addr default = %q; want 127.0.0.1:7777", f.DefValue)
	}
	if f := c.Flags().Lookup("no-tailscale"); f == nil {
		t.Fatal("--no-tailscale flag missing")
	} else if f.DefValue != "false" {
		t.Errorf("--no-tailscale default = %q; want false", f.DefValue)
	}
}

func TestACPBridgeCmd_RefusesWithoutNoTailscale(t *testing.T) {
	c := NewACPBridgeCmd()
	c.SetArgs([]string{"--addr", "127.0.0.1:0"})
	var stderr bytes.Buffer
	c.SetErr(&stderr)
	c.SilenceUsage = true
	err := c.Execute()
	if err == nil {
		t.Fatal("expected error when --no-tailscale is not set")
	}
	if !strings.Contains(err.Error(), "no-tailscale") {
		t.Errorf("error %q does not mention --no-tailscale", err)
	}
}
