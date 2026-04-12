package logger

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type captureHandler struct {
	enabled bool
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return h.enabled }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func TestMultiplexHandlerEnabled(t *testing.T) {
	m := &MultiplexHandler{
		handlers: []slog.Handler{
			&captureHandler{enabled: false},
			&captureHandler{enabled: true},
		},
	}
	if !m.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected Enabled to be true when any child handler is enabled")
	}
}

func TestMultiplexHandlerHandleWritesToEnabledHandlers(t *testing.T) {
	enabled := &captureHandler{enabled: true}
	disabled := &captureHandler{enabled: false}
	m := &MultiplexHandler{
		handlers: []slog.Handler{enabled, disabled},
	}

	record := slog.NewRecord(testTime(), slog.LevelInfo, "hello", 0)
	if err := m.Handle(context.Background(), record); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if len(enabled.records) != 1 {
		t.Fatalf("expected enabled handler to receive 1 record, got %d", len(enabled.records))
	}
	if len(disabled.records) != 0 {
		t.Fatalf("expected disabled handler to receive 0 records, got %d", len(disabled.records))
	}
}

func testTime() time.Time {
	return time.Unix(0, 0)
}
