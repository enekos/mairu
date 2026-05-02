package acpbridge

import (
	"net"
	"testing"
)

func TestAllowAllAlwaysOK(t *testing.T) {
	if _, err := (AllowAll{}).Authorize(nil); err != nil {
		t.Fatalf("AllowAll returned error: %v", err)
	}
}

func TestPeerAuthorizerInterface(t *testing.T) {
	var _ PeerAuthorizer = AllowAll{}
	var _ PeerAuthorizer = TailscaleAuth{}
}

func TestTailscaleAuthForwardsIdentity(t *testing.T) {
	a := TailscaleAuth{WhoIs: func(remote string) (string, error) { return "user@tailnet", nil }}
	p, err := a.Authorize(&net.TCPAddr{IP: net.ParseIP("100.64.0.1"), Port: 1234})
	if err != nil {
		t.Fatal(err)
	}
	if p.Identity != "user@tailnet" {
		t.Fatalf("identity = %q", p.Identity)
	}
}
