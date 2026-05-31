package acpbridge

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// pingInterval governs how often we send a WebSocket ping to detect dead
// peers. The default value is generous enough not to thrash mobile radios but
// tight enough that a stale tab disconnects within a minute.
const pingInterval = 25 * time.Second

// stamp inserts "x-mairu-event-id":<id> at the start of a JSON object frame.
// If the frame is not a JSON object (defensive), the id is wrapped into a
// pass-through envelope so clients still get a monotonic id for replay.
func stamp(frame []byte, id uint64) []byte {
	trimmed := bytes.TrimSpace(frame)
	if len(trimmed) >= 2 && trimmed[0] == '{' {
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
	// Wrap non-object frames so the event id is never lost. ACP agents only
	// emit JSON-RPC objects per protocol, so this branch is defensive — but
	// silently dropping the id would break Last-Event-ID replay if it ever
	// happened.
	out := make([]byte, 0, len(frame)+48)
	out = append(out, []byte(`{"x-mairu-event-id":`)...)
	out = strconv.AppendUint(out, id, 10)
	out = append(out, []byte(`,"raw":`)...)
	encoded, _ := encodeJSONString(string(frame))
	out = append(out, encoded...)
	out = append(out, '}')
	return out
}

// encodeJSONString wraps s as a JSON string literal.
func encodeJSONString(s string) ([]byte, error) {
	// Hand-rolled to avoid pulling in encoding/json for hot paths; only ASCII
	// double-quote/backslash/control chars need escaping for our purposes.
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if c < 0x20 {
				out = append(out, '\\', 'u', '0', '0',
					hexChar(c>>4), hexChar(c&0xF))
			} else {
				out = append(out, c)
			}
		}
	}
	out = append(out, '"')
	return out, nil
}

func hexChar(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + (n - 10)
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

	// Accept Last-Event-ID via either the standard EventSource header or the
	// `last_event_id` query parameter. Mobile clients on platforms that don't
	// expose WebSocket headers (notably React Native) can only set query
	// parameters; refusing to honor that breaks reconnect-replay for them.
	var lastEventID uint64
	if h := r.Header.Get("Last-Event-ID"); h != "" {
		if n, err := strconv.ParseUint(h, 10, 64); err == nil {
			lastEventID = n
		}
	}
	if lastEventID == 0 {
		if q := r.URL.Query().Get("last_event_id"); q != "" {
			if n, err := strconv.ParseUint(q, 10, 64); err == nil {
				lastEventID = n
			}
		}
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusInternalError, "closing")
	if b.opts.MaxFrameBytes > 0 {
		c.SetReadLimit(b.opts.MaxFrameBytes)
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	sub := sess.Subscribe()
	defer sess.Unsubscribe(sub)

	// Replay missed frames first (from the per-session ring).
	for _, sf := range b.registry.replay(id, lastEventID) {
		wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
		err := c.Write(wctx, websocket.MessageText, stamp(sf.Frame, sf.ID))
		wcancel()
		if err != nil {
			return
		}
	}

	// Fan out new frames + pump client→agent + heartbeat.
	errCh := make(chan error, 3)
	// Writer goroutine: also handles ping ticks and session-done propagation.
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case <-sess.Done():
				// Tell the client why the session went away. Best-effort.
				if msg := sess.ExitMessage(); msg != "" {
					notif := []byte(`{"jsonrpc":"2.0","method":"$/sessionExit","params":{"reason":` + jsonString(msg) + `}}`)
					wctx, wcancel := context.WithTimeout(context.Background(), 2*time.Second)
					_ = c.Write(wctx, websocket.MessageText, notif)
					wcancel()
				}
				errCh <- errors.New("session ended")
				return
			case <-ticker.C:
				pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
				err := c.Ping(pctx)
				pcancel()
				if err != nil {
					errCh <- err
					return
				}
			case sf, ok := <-sub:
				if !ok {
					errCh <- errors.New("subscription closed")
					return
				}
				wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
				err := c.Write(wctx, websocket.MessageText, stamp(sf.Frame, sf.ID))
				wcancel()
				if err != nil {
					errCh <- err
					return
				}
			}
		}
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

// jsonString encodes s as a JSON string literal (with surrounding quotes).
func jsonString(s string) string {
	enc, _ := encodeJSONString(s)
	return string(enc)
}

func remoteAddr(r *http.Request) addr { return addr(r.RemoteAddr) }

type addr string

func (a addr) Network() string { return "tcp" }
func (a addr) String() string  { return string(a) }

// compile guard: ensure net.Addr is satisfied by addr.
var _ net.Addr = addr("")
