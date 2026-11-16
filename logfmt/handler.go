// Package logfmt implements a custom slog.Handler that formats log messages
// in a Zorp-inspired log format:
//
//	<iso timestamp> <component>.<severity>(<sessionId>): <message>; key='value', key2='value2'
//
// The session ID is extracted from context.Context (set by the session package
// per-connection), so all components logging with that context automatically
// get the correct session ID without explicit attribute passing.
package logfmt

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/endreszabo/zorplogger/session"
)

// Handler implements slog.Handler with a Zorp-inspired custom log format.
type Handler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Level
	attrs []slog.Attr // pre-collected attributes from WithAttrs
	group string      // current group prefix
}

// New creates a new Handler that writes to w at the given minimum level.
func New(w io.Writer, level slog.Level) *Handler {
	return &Handler{
		w:     w,
		mu:    &sync.Mutex{},
		level: level,
	}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// Extract session ID from context
	sid := session.FromContext(ctx)
	if sid == "" {
		sid = "n/a"
	}

	// Collect all attributes: pre-set attrs + record attrs
	var component string
	var fields []field

	// Process pre-set attrs (from With())
	for _, a := range h.attrs {
		key := h.prefixKey(a.Key)
		switch key {
		case "component":
			component = a.Value.String()
		case "sid":
			// Skip — we get it from context now
		default:
			fields = append(fields, field{key: key, value: formatValue(a.Value)})
		}
	}

	// Process record attrs
	r.Attrs(func(a slog.Attr) bool {
		key := h.prefixKey(a.Key)
		switch key {
		case "component":
			component = a.Value.String()
		case "sid":
			// Skip — we get it from context
		default:
			fields = append(fields, field{key: key, value: formatValue(a.Value)})
		}
		return true
	})

	// Defaults
	if component == "" {
		component = "core"
	}

	// Format severity
	severity := strings.ToLower(r.Level.String())

	// Format timestamp
	timestamp := r.Time.Format(time.RFC3339)

	// Build the line
	var buf strings.Builder
	fmt.Fprintf(&buf, "%s %s.%s(%s): %s", timestamp, component, severity, sid, r.Message)

	// Append fields
	if len(fields) > 0 {
		buf.WriteString("; ")
		for i, f := range fields {
			if i > 0 {
				buf.WriteString(", ")
			}
			fmt.Fprintf(&buf, "%s='%s'", f.key, f.value)
		}
	}
	buf.WriteByte('\n')

	// Write atomically
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, buf.String())
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	newAttrs = append(newAttrs, attrs...)
	return &Handler{
		w:     h.w,
		mu:    h.mu, // share mutex across all derived handlers
		level: h.level,
		attrs: newAttrs,
		group: h.group,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	newAttrs := make([]slog.Attr, len(h.attrs))
	copy(newAttrs, h.attrs)
	return &Handler{
		w:     h.w,
		mu:    h.mu,
		level: h.level,
		attrs: newAttrs,
		group: newGroup,
	}
}

func (h *Handler) prefixKey(key string) string {
	if h.group != "" {
		return h.group + "." + key
	}
	return key
}

type field struct {
	key   string
	value string
}

func formatValue(v slog.Value) string {
	switch v.Kind() {
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindDuration:
		return v.Duration().String()
	default:
		return v.String()
	}
}
