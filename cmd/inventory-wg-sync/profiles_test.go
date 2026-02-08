package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetermineIPCapability(t *testing.T) {
	ipv4, ipv6 := determineIPCapability([]string{
		"[Interface]",
		"Address = 10.0.0.1/32",
	})
	if !ipv4 || ipv6 {
		t.Fatalf("determineIPCapability(ipv4) = %v,%v, want true,false", ipv4, ipv6)
	}

	ipv4, ipv6 = determineIPCapability([]string{
		"[Interface]",
		"Address = fd00::1/128",
	})
	if ipv4 || !ipv6 {
		t.Fatalf("determineIPCapability(ipv6) = %v,%v, want false,true", ipv4, ipv6)
	}
}

func TestUpdateWGProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.conf")
	initial := strings.Join([]string{
		"[Interface]",
		"Address = 10.0.0.1/32",
		"Table = 123",
		"PostUp = old",
		"PostDown = old",
		"",
		"[Peer]",
		"AllowedIPs = 10.0.0.0/8",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	allowed := []string{"10.0.0.1/32", "fd00::1/128"}
	postUp := []string{"echo up"}
	postDown := []string{"echo down"}
	if err := updateWGProfile("wg0", allowed, postUp, postDown, path, 555); err != nil {
		t.Fatalf("updateWGProfile() error = %v", err)
	}

	gotb, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(gotb)

	if !strings.Contains(got, "AllowedIPs = 10.0.0.1/32") {
		t.Fatalf("profile missing filtered AllowedIPs: %q", got)
	}
	if strings.Contains(got, "fd00::1/128") {
		t.Fatalf("profile contains IPv6 when ipv4-only profile: %q", got)
	}
	if !strings.Contains(got, "Table = 555") {
		t.Fatalf("profile missing updated Table: %q", got)
	}
	if !strings.Contains(got, "PostUp = echo up") {
		t.Fatalf("profile missing updated PostUp: %q", got)
	}
	if !strings.Contains(got, "PostDown = echo down") {
		t.Fatalf("profile missing updated PostDown: %q", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file perms = %v, want 0600", info.Mode().Perm())
	}
}
