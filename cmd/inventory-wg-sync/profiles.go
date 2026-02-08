package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/etkecc/inventory-wg-sync/internal/config"
)

func handleWireGuard(cfg *config.Config, allowedIPs, postUp, postDown []string) error {
	if cfg.ProfilePath == "" {
		return nil
	}

	logger.Println("updating WireGuard profle", cfg.ProfilePath)
	name := strings.Replace(filepath.Base(cfg.ProfilePath), filepath.Ext(cfg.ProfilePath), "", 1)
	if err := updateWGProfile(name, allowedIPs, postUp, postDown, cfg.ProfilePath, cfg.Table); err != nil {
		return err
	}

	logger.Println("restarting WireGuard interface", name)

	// If the interface doesn't exist, start the instantiated systemd service.
	//
	// Otherwise, restart it fully.
	// Reloading (which uses `wg syncconf`) is less disruptive, but doesn't apply `AllowedIPs` changes.

	if err := exec.Command("wg", "show", name).Run(); err != nil {
		return exec.Command("systemctl", "start", "wg-quick@"+name).Run() //nolint:gosec // that's ok
	}
	return exec.Command("systemctl", "restart", "wg-quick@"+name).Run() //nolint:gosec // that's ok
}

func updateWGProfile(name string, allowedIPs, postUp, postDown []string, path string, table int) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(contents), "\n")
	allowedIPs = filterOutUnsupportedIPs(lines, allowedIPs)

	for i, line := range lines {
		if strings.HasPrefix(line, "Table") && table > 0 {
			lines[i] = "Table = " + strconv.Itoa(table)
		}
		if strings.HasPrefix(line, "AllowedIPs") {
			lines[i] = "AllowedIPs = " + strings.Join(allowedIPs, ",")
		}
		if strings.HasPrefix(line, "PostUp") && len(postUp) > 0 {
			lines[i] = "PostUp = " + strings.Join(postUp, "; ")
		}
		if strings.HasPrefix(line, "PostDown") && len(postDown) > 0 {
			lines[i] = "PostDown = " + strings.Join(postDown, "; ")
		}
	}

	contents, err = applyVars(strings.Join(lines, "\n"), map[string]any{"name": name, "table": table})
	if err != nil {
		return err
	}

	return os.WriteFile(path, contents, 0o600)
}

// determineIPCapability tells if the WireGuard profile contains IPv4 and IPv6 addresses on the `Interface.Address` line
func determineIPCapability(lines []string) (ipv4, ipv6 bool) {
	for _, line := range lines {
		if strings.HasPrefix(line, "Address") {
			isIPv4Capable := strings.Contains(line, ".")
			isIPv6Capable := strings.Contains(line, ":")
			return isIPv4Capable, isIPv6Capable
		}
	}
	return false, false
}

// filterOutUnsupportedIPs filters out IP addresses that the WireGuard profile does not support
func filterOutUnsupportedIPs(lines, allowedIPs []string) []string {
	ipv4, ipv6 := determineIPCapability(lines)
	if !ipv4 {
		allowedIPsNew := filterOutCIDRsContainingChar(allowedIPs, ".")
		diff := len(allowedIPs) - len(allowedIPsNew)
		if diff > 0 {
			logger.Println("filtered out", diff, "IPv4 CIDRs due to the profile's lack of IPv4 support")
			allowedIPs = allowedIPsNew
		}
	}
	if !ipv6 {
		allowedIPsNew := filterOutCIDRsContainingChar(allowedIPs, ":")
		diff := len(allowedIPs) - len(allowedIPsNew)
		if diff > 0 {
			logger.Println("filtered out", diff, "IPv6 CIDRs due to the profile's lack of IPv6 support")
			allowedIPs = allowedIPsNew
		}
	}

	return allowedIPs
}

// filterOutCIDRsContainingChar removes CIDRs from the allowedIPs list if they contain a given character (e.g. "." for IPv4 and ":" for IPv6)
func filterOutCIDRsContainingChar(allowedIPs []string, needle string) []string {
	result := make([]string, 0, len(allowedIPs))
	for _, ip := range allowedIPs {
		if !strings.Contains(ip, needle) {
			result = append(result, ip)
		}
	}
	return result
}

func applyVars(tplString string, vars map[string]any) ([]byte, error) {
	var result bytes.Buffer
	tpl, err := template.New("template").Parse(tplString)
	if err != nil {
		return nil, err
	}
	err = tpl.Execute(&result, vars)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}
