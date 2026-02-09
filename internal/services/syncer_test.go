package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/etkecc/inventory-wg-sync/internal/models"
)

func TestSync_NoAllowedIPs(t *testing.T) {
	cfg := &models.Config{}
	if err := Sync(cfg); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}

func TestSync_WithAllowedIPs_NoProfile(t *testing.T) {
	cfg := &models.Config{
		AllowedIPs:  []string{"1.2.3.4"},
		ProfilePath: "",
	}
	if err := Sync(cfg); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}

func TestSync_WithInventory(t *testing.T) {
	dir := t.TempDir()
	invPath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(invPath, []byte("host1 ansible_host=1.2.3.4\nhost2 ansible_host=10.0.0.0/8\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := &models.Config{
		InventoryPaths: []string{invPath},
		ProfilePath:    "",
	}
	if err := Sync(cfg); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
}
