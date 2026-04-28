package acpbridge

import "net"

// Peer represents an authenticated remote peer.
type Peer struct{ Identity string }

// PeerAuthorizer decides whether a remote address is allowed to connect.
type PeerAuthorizer interface {
	Authorize(remote net.Addr) (Peer, error)
}

// AllowAll is a no-op authorizer used in tests and local-only deployments.
type AllowAll struct{}

func (AllowAll) Authorize(net.Addr) (Peer, error) { return Peer{Identity: "anonymous"}, nil }
