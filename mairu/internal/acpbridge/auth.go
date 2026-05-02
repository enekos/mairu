package acpbridge

import (
	"errors"
	"net"
)

// Peer represents an authenticated remote peer.
type Peer struct{ Identity string }

// PeerAuthorizer decides whether a remote address is allowed to connect.
type PeerAuthorizer interface {
	Authorize(remote net.Addr) (Peer, error)
}

// AllowAll is a no-op authorizer used in tests and local-only deployments.
type AllowAll struct{}

func (AllowAll) Authorize(net.Addr) (Peer, error) { return Peer{Identity: "anonymous"}, nil }

// TailscaleAuth is a PeerAuthorizer that delegates identity lookup to a
// pluggable WhoIs function (typically backed by tsnet.Server.LocalClient()
// .WhoIs). Construct directly: TailscaleAuth{WhoIs: ...}.
type TailscaleAuth struct {
	WhoIs func(remote string) (string, error)
}

func (t TailscaleAuth) Authorize(remote net.Addr) (Peer, error) {
	if remote == nil {
		return Peer{}, errors.New("no remote address")
	}
	id, err := t.WhoIs(remote.String())
	if err != nil {
		return Peer{}, err
	}
	return Peer{Identity: id}, nil
}
