// Package sealed encrypts small values with AES-256-GCM (ADR-0029): the tokens a console session
// stores in Postgres, and the short-lived cookie that carries the state of a sign-in. A sealed
// value cannot be read or changed without the key; tampering makes Open fail.
//
// Keys rotate without signing anyone out: the first key seals, every key opens, so a new key goes
// first and the old one stays until the values it sealed have expired.
package sealed

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Box seals and opens values.
type Box struct {
	aeads []cipher.AEAD
}

// ErrOpen means a value was not sealed by any of the keys, or was changed since.
var ErrOpen = errors.New("sealed: the value cannot be opened")

// New reads keys written as standard base64 of 32 bytes each, separated by commas, the first one
// sealing.
func New(keys string) (*Box, error) {
	var box Box
	for _, k := range strings.Split(keys, ",") {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(k))
		if err != nil || len(raw) != 32 {
			return nil, errors.New("sealed: every key must be 32 bytes in standard base64 (openssl rand -base64 32)")
		}
		block, err := aes.NewCipher(raw)
		if err != nil {
			return nil, fmt.Errorf("sealed: %w", err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("sealed: %w", err)
		}
		box.aeads = append(box.aeads, aead)
	}
	return &box, nil
}

// Seal encrypts plaintext with the first key. The output is the random nonce followed by the
// ciphertext and its tag.
func (b *Box) Seal(plaintext []byte) []byte {
	aead := b.aeads[0]
	nonce := make([]byte, aead.NonceSize())
	// crypto/rand.Read never fails on supported platforms (Go 1.24 and later).
	_, _ = rand.Read(nonce)
	return aead.Seal(nonce, nonce, plaintext, nil)
}

// Open decrypts a value sealed by any of the keys.
func (b *Box) Open(sealed []byte) ([]byte, error) {
	for _, aead := range b.aeads {
		n := aead.NonceSize()
		if len(sealed) < n {
			return nil, ErrOpen
		}
		if out, err := aead.Open(nil, sealed[:n], sealed[n:], nil); err == nil {
			return out, nil
		}
	}
	return nil, ErrOpen
}

// SealString seals text and returns it in URL-safe base64, for a cookie.
func (b *Box) SealString(text string) string {
	return base64.RawURLEncoding.EncodeToString(b.Seal([]byte(text)))
}

// OpenString opens a value made by SealString.
func (b *Box) OpenString(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", ErrOpen
	}
	out, err := b.Open(raw)
	return string(out), err
}
