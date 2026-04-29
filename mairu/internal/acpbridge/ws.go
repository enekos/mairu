package acpbridge

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// stamp inserts "x-mairu-event-id":<id> at the start of a JSON object frame.
// If the frame is not a JSON object (defensive), it returns the frame unchanged.
func stamp(frame []byte, id uint64) []byte {
	trimmed := bytes.TrimSpace(frame)
	if len(trimmed) < 2 || trimmed[0] != '{' {
		return frame
	}
	out := make([]byte, 0, len(trimmed)+24)
	out = append(out, '{')
	out = append(out, []byte(`"x-mairu-event-id":`)...)
	out = strconv.AppendUint(out, id, 10)
	if trimmed[1] != '}' {
		out = append(out, ',')
	}
	out = append(out, trimmed[1:]...)
	return out
}

func (b *Bridge) handleWS(w http.ResponseWriter, r *http.Request) {
	if _, err := b.opts.Authorizer.Authorize(remoteAddr(r)); err != nil {
		http.Error(w, "forbidden: "+err.Error(), 403)
		return
	}

	id := r.URL.Query().Get("session")
	if id == "" {
		id = b.registry.Newest()
	}
	sess, ok := b.registry.Get(id)
	if !ok {
		http.Error(w, "no such session", 404)
		return
	}

	var lastEventID uint64
	if h := r.Header.Get("Last-Event-ID"); h != "" {
		if n, err := strconv.ParseUint(h, 10, 64); err == nil {
			lastEventID = n
		}
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "closing")

	ctx := r.Context()
	sub := sess.Subscribe()
	defer sess.Unsubscribe(sub)

	// Replay missed frames first (from the per-session ring).
	for _, sf := range b.registry.replay(id, lastEventID) {
		if err := c.Write(ctx, websocket.MessageText, stamp(sf.Frame, sf.ID)); err != nil {
			return
		}
	}

	// Fan out new frames + pump client→agent.
	errCh := make(chan error, 2)
	go func() {
		for sf := range sub {
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.Write(wctx, websocket.MessageText, stamp(sf.Frame, sf.ID))
			cancel()
			if err != nil {
				errCh <- err
				return
			}
		}
		errCh <- nil
	}()
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			b.registry.TouchActivity(id)
			if err := sess.Send(data); err != nil {
				errCh <- err
				return
			}
		}
	}()
	<-errCh
	c.Close(websocket.StatusNormalClosure, "")
}

func remoteAddr(r *http.Request) addr { return addr(r.RemoteAddr) }

type addr string

func (a addr) Network() string { return "tcp" }
func (a addr) String() string  { return string(a) }

// compile guard: ensure net.Addr is satisfied by addr.
var _ net.Addr = addr("")
