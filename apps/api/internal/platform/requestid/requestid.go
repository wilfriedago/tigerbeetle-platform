// Package requestid carries the request ID of a call through its context. The HTTP middleware
// takes it from the X-Request-Id header or makes one; every layer reads it back for logs, stored
// write records and problem details.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// Header carries the request ID in and out of HTTP requests.
const Header = "X-Request-Id"

// maxLength bounds a client-supplied ID, so it can be logged and echoed as is.
const maxLength = 128

// contextKey is unexported so no other package can read or overwrite the value by accident.
type contextKey struct{}

// With returns a copy of ctx that carries id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// From returns the request ID carried by ctx, or "" when there is none.
func From(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}

// Valid reports whether a client-supplied ID can be kept: 1 to 128 printable ASCII characters
// without spaces. Anything else is replaced, so distributed traces line up when clients behave
// and logs stay clean when they do not.
func Valid(id string) bool {
	if id == "" || len(id) > maxLength {
		return false
	}
	for _, c := range id {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}

// New returns a random ID of 32 lowercase hexadecimal characters.
func New() string {
	var b [16]byte
	// crypto/rand never fails on supported platforms (Go 1.24+); it panics otherwise.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
