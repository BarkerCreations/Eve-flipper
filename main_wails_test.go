//go:build wails
// +build wails

package main

import (
	"net"
	"strconv"
	"testing"
)

func TestListenOnPreferredOrFreePortUsesPreferredPort(t *testing.T) {
	reserved, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	preferredPort := reserved.Addr().(*net.TCPAddr).Port
	if err := reserved.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}

	ln, port, err := listenOnPreferredOrFreePort("127.0.0.1", preferredPort)
	if err != nil {
		t.Fatalf("listen on preferred port: %v", err)
	}
	defer ln.Close()

	if port != preferredPort {
		t.Fatalf("expected preferred port %d, got %d", preferredPort, port)
	}
}

func TestListenOnPreferredOrFreePortFallsBackWhenPreferredPortIsBusy(t *testing.T) {
	occupied, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		t.Fatalf("occupy preferred port: %v", err)
	}
	defer occupied.Close()
	preferredPort := occupied.Addr().(*net.TCPAddr).Port

	ln, port, err := listenOnPreferredOrFreePort("127.0.0.1", preferredPort)
	if err != nil {
		t.Fatalf("listen with fallback: %v", err)
	}
	defer ln.Close()

	if port == preferredPort {
		t.Fatalf("expected fallback port, got occupied preferred port %d", preferredPort)
	}
	if ln.Addr().String() == net.JoinHostPort("127.0.0.1", strconv.Itoa(preferredPort)) {
		t.Fatalf("listener unexpectedly bound occupied preferred address %s", ln.Addr())
	}
}
