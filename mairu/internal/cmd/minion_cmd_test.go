package cmd

import "testing"

func TestNewMinionCmd_RegistersCouncilFlag(t *testing.T) {
	minionCouncil = false
	c := NewMinionCmd()
	f := c.Flags().Lookup("council")
	if f == nil {
		t.Fatalf("expected --council flag to be registered")
	}
}
