//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestDPAPISecretsRoundTrip(t *testing.T) {
	secret := "private-test-12345"
	enc, err := encryptSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, dpapiPrefix) || strings.Contains(enc, secret) {
		t.Fatal("secret not encrypted")
	}
	dec, err := decryptSecret(enc)
	if err != nil || dec != secret {
		t.Fatalf("DPAPI round trip: %v", err)
	}
}
