package main

import (
	"reflect"
	"testing"
)

func TestTrustedProxiesDefaultsToLoopback(t *testing.T) {
	t.Setenv("APIG0_TRUSTED_PROXIES", "")

	got := trustedProxies()
	want := []string{"127.0.0.1", "::1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("trusted proxies mismatch: got %#v, want %#v", got, want)
	}
}

func TestTrustedProxiesParsesEnv(t *testing.T) {
	t.Setenv("APIG0_TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8,,192.168.1.10")

	got := trustedProxies()
	want := []string{"127.0.0.1", "10.0.0.0/8", "192.168.1.10"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("trusted proxies mismatch: got %#v, want %#v", got, want)
	}
}
