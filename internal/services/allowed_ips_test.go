package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/etkecc/inventory-wg-sync/internal/models"
)

func TestConfigIPs(t *testing.T) {
	cfg := &models.Config{
		AllowedIPs:  []string{"1.2.3.4", "10.0.0.0/8", "bad_host"},
		ExcludedIPs: []string{"1.2.3.4", "10.0.0.0/8", "also_bad"},
	}
	allowed, excluded := configIPs(cfg)
	if len(allowed) != 0 {
		t.Fatalf("configIPs() allowed = %#v, want empty", allowed)
	}
	if !excluded["1.2.3.4/32"] || !excluded["10.0.0.0/8"] {
		t.Fatalf("configIPs() excluded missing expected entries: %#v", excluded)
	}
}

func TestAllowedIPs_WithInventoryAndExclusions(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(invPath, []byte("host1 ansible_host=1.2.3.4\nhost2 ansible_host=10.0.0.0/8\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := &models.Config{
		InventoryPaths: []string{invPath, filepath.Join(dir, "missing")},
		AllowedIPs:     []string{"10.1.1.1"},
		ExcludedIPs:    []string{"1.2.3.4"},
	}
	got := AllowedIPs(cfg)
	if len(got) != 2 {
		t.Fatalf("AllowedIPs() = %#v, want 2 entries", got)
	}
}

func TestInventoryIPs_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(invPath, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got := inventoryIPs(invPath, map[string]bool{})
	if got != nil {
		t.Fatalf("inventoryIPs() = %#v, want nil", got)
	}
}

func TestInventoryIPs_MissingFile(t *testing.T) {
	got := inventoryIPs(filepath.Join(t.TempDir(), "missing"), map[string]bool{})
	if got != nil {
		t.Fatalf("inventoryIPs() = %#v, want nil", got)
	}
}

func TestHostAllowedIPs_Excluded(t *testing.T) {
	excluded := map[string]bool{"10.0.0.1/32": true}
	got := hostAllowedIPs("10.0.0.1", excluded)
	if len(got) != 0 {
		t.Fatalf("hostAllowedIPs() = %#v, want empty", got)
	}
}

func TestHostAllowedIPs_InvalidHost(t *testing.T) {
	got := hostAllowedIPs("bad_host", map[string]bool{})
	if got != nil {
		t.Fatalf("hostAllowedIPs() = %#v, want nil", got)
	}
}
