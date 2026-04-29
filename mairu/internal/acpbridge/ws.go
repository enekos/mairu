package acpbridge

import "net/http"

// handleWS is implemented in Task 8.
func (b *Bridge) handleWS(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "ws not yet implemented", 501)
}
