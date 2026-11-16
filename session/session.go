// Package session provides lightweight session ID generation
// for correlating log messages across a connection's lifecycle.
// The session ID is stored in context.Context and extracted by the
// log handler automatically.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
)

// counter provides a monotonic fallback if crypto/rand fails.
var counter atomic.Uint64

// contextKey is the unexported key type for session IDs in context.
type contextKey struct{}

// ID generates a short unique session identifier (8 hex chars).
func ID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback: monotonic counter
		n := counter.Add(1)
		b[0] = byte(n >> 24)
		b[1] = byte(n >> 16)
		b[2] = byte(n >> 8)
		b[3] = byte(n)
	}
	return hex.EncodeToString(b)
}

// NewContext returns a new context with a freshly generated session ID.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, ID())
}

// WithID returns a new context with the given session ID.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext extracts the session ID from the context.
// Returns empty string if no session ID is set.
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}
