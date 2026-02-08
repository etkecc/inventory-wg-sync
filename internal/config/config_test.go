package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	contents := `
inventory_paths:
  - /etc/ansible/hosts
profile_path: /etc/wireguard/wg0.conf
allowed_ips:
  - 10.0.0.0/8
excluded_ips:
  - 10.10.0.0/16
table: 1234
post_up:
  - echo up
post_down:
  - echo down
debug: true
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	want := &Config{
		InventoryPaths: []string{"/etc/ansible/hosts"},
		ProfilePath:    "/etc/wireguard/wg0.conf",
		AllowedIPs:     []string{"10.0.0.0/8"},
		ExcludedIPs:    []string{"10.10.0.0/16"},
		Table:          1234,
		PostUp:         []string{"echo up"},
		PostDown:       []string{"echo down"},
		Debug:          true,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read() = %#v, want %#v", got, want)
	}
}
