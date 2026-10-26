package sealed_test

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/sealed"
)

func key(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestSealOpenAndRotate(t *testing.T) {
	oldKey, newKey := key(t), key(t)
	before, err := sealed.New(oldKey)
	if err != nil {
		t.Fatal(err)
	}
	value := before.SealString("refresh-token")

	after, err := sealed.New(newKey + "," + oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := after.OpenString(value); err != nil || got != "refresh-token" {
		t.Fatalf("old value under rotated keys = %q, %v", got, err)
	}
	fresh := after.SealString("x")
	if _, err := before.OpenString(fresh); !errors.Is(err, sealed.ErrOpen) {
		t.Fatalf("a value sealed with the new key opened with the old one: %v", err)
	}
	tampered := []byte(value)
	tampered[len(tampered)-1] ^= 1
	if _, err := after.OpenString(string(tampered)); !errors.Is(err, sealed.ErrOpen) {
		t.Fatalf("tampered value = %v, want ErrOpen", err)
	}
	if _, err := sealed.New("short"); err == nil {
		t.Fatal("a short key was accepted")
	}
}
