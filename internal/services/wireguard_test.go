package services

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/etkecc/inventory-wg-sync/internal/models"
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

func TestFilterOutCIDRsContainingChar(t *testing.T) {
	ips := []string{"10.0.0.1/32", "fd00::1/128", "10.0.0.2/32"}
	got := filterOutCIDRsContainingChar(ips, ".")
	want := []string{"fd00::1/128"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterOutCIDRsContainingChar() = %#v, want %#v", got, want)
	}
}

func TestDetermineIPCapability_Both(t *testing.T) {
	ipv4, ipv6 := determineIPCapability([]string{
		"[Interface]",
		"Address = 10.0.0.1/32, fd00::1/128",
	})
	if !ipv4 || !ipv6 {
		t.Fatalf("determineIPCapability(both) = %v,%v, want true,true", ipv4, ipv6)
	}
}

func TestFilterOutUnsupportedIPs_NoAddress(t *testing.T) {
	allowed := []string{"10.0.0.1/32", "fd00::1/128"}
	got := filterOutUnsupportedIPs([]string{"[Interface]"}, allowed)
	if len(got) != 0 {
		t.Fatalf("filterOutUnsupportedIPs(no-address) = %#v, want empty", got)
	}
}

func TestUpdateWGProfile_NoTableNoPost(t *testing.T) {
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
		"Endpoint = {{ .name }}",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	allowed := []string{"10.0.0.1/32"}
	if err := updateWGProfile("wg0", allowed, nil, nil, path, 0); err != nil {
		t.Fatalf("updateWGProfile() error = %v", err)
	}

	gotb, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(gotb)

	if !strings.Contains(got, "Table = 123") {
		t.Fatalf("profile missing original Table: %q", got)
	}
	if !strings.Contains(got, "PostUp = old") {
		t.Fatalf("profile missing original PostUp: %q", got)
	}
	if !strings.Contains(got, "PostDown = old") {
		t.Fatalf("profile missing original PostDown: %q", got)
	}
	if !strings.Contains(got, "Endpoint = wg0") {
		t.Fatalf("profile missing template expansion: %q", got)
	}
}

func TestUpdateWGProfile_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.conf")
	initial := strings.Join([]string{
		"[Interface]",
		"Address = 10.0.0.1/32",
		"Endpoint = {{ .name",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := updateWGProfile("wg0", []string{"10.0.0.1/32"}, nil, nil, path, 0); err == nil {
		t.Fatalf("updateWGProfile() expected error for invalid template")
	}
}

func TestRunSystemctl_EmptyName(t *testing.T) {
	if err := runSystemctl("start", ""); err == nil {
		t.Fatalf("runSystemctl() expected error for empty name")
	}
}

func TestRunSystemctl_PathMissing(t *testing.T) {
	t.Setenv("PATH", "")
	if err := runSystemctl("start", "wg0"); err == nil {
		t.Fatalf("runSystemctl() expected error when systemctl is missing")
	}
}

func TestUpdateWGProfile_ReadFileError(t *testing.T) {
	if err := updateWGProfile("wg0", []string{"10.0.0.1/32"}, nil, nil, filepath.Join(t.TempDir(), "missing.conf"), 0); err == nil {
		t.Fatalf("updateWGProfile() expected error for missing file")
	}
}

func TestApplyVars_ExecuteError(t *testing.T) {
	if _, err := applyVars("{{ call .name }}", map[string]any{"name": "wg0"}); err == nil {
		t.Fatalf("applyVars() expected execute error")
	}
}

func TestSyncWireGuard_ProfileReadError(t *testing.T) {
	cfg := &models.Config{ProfilePath: filepath.Join(t.TempDir(), "wg0.conf")}
	if err := SyncWireGuard(cfg, []string{"10.0.0.1/32"}); err == nil {
		t.Fatalf("SyncWireGuard() expected error for missing profile")
	}
}

func TestSyncWireGuard_NoProfile(t *testing.T) {
	cfg := &models.Config{}
	if err := SyncWireGuard(cfg, []string{"10.0.0.1/32"}); err != nil {
		t.Fatalf("SyncWireGuard() error = %v", err)
	}
}

func TestSyncWireGuard_InvalidInterfaceName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wg!0.conf")
	if err := os.WriteFile(path, []byte("[Interface]\nAddress = 10.0.0.1/32\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := &models.Config{ProfilePath: path}
	if err := SyncWireGuard(cfg, []string{"10.0.0.1/32"}); err == nil {
		t.Fatalf("SyncWireGuard() expected error for invalid interface name")
	}
}

func TestSyncWireGuard_StartsWhenInterfaceMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.conf")
	if err := os.WriteFile(path, []byte("[Interface]\nAddress = 10.0.0.1/32\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origInterfaceByName := interfaceByName
	origRunSystemctl := runSystemctlFunc
	t.Cleanup(func() {
		interfaceByName = origInterfaceByName
		runSystemctlFunc = origRunSystemctl
	})

	interfaceByName = func(string) (*net.Interface, error) {
		return nil, errors.New("not found")
	}
	var gotAction string
	runSystemctlFunc = func(action, _ string) error {
		gotAction = action
		return nil
	}

	cfg := &models.Config{ProfilePath: path}
	if err := SyncWireGuard(cfg, []string{"10.0.0.1/32"}); err != nil {
		t.Fatalf("SyncWireGuard() error = %v", err)
	}
	if gotAction != "start" {
		t.Fatalf("systemctl action = %q, want %q", gotAction, "start")
	}
}

func TestSyncWireGuard_RestartsWhenInterfaceExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wg0.conf")
	if err := os.WriteFile(path, []byte("[Interface]\nAddress = 10.0.0.1/32\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origInterfaceByName := interfaceByName
	origRunSystemctl := runSystemctlFunc
	t.Cleanup(func() {
		interfaceByName = origInterfaceByName
		runSystemctlFunc = origRunSystemctl
	})

	interfaceByName = func(string) (*net.Interface, error) {
		return &net.Interface{}, nil
	}
	var gotAction string
	runSystemctlFunc = func(action, _ string) error {
		gotAction = action
		return nil
	}

	cfg := &models.Config{ProfilePath: path}
	if err := SyncWireGuard(cfg, []string{"10.0.0.1/32"}); err != nil {
		t.Fatalf("SyncWireGuard() error = %v", err)
	}
	if gotAction != "restart" {
		t.Fatalf("systemctl action = %q, want %q", gotAction, "restart")
	}
}
