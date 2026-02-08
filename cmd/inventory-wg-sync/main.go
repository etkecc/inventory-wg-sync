package main

import (
	"bytes"
	"log"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/adrg/xdg"
	"github.com/etkecc/go-ansible"
	"github.com/etkecc/go-kit"

	"github.com/etkecc/inventory-wg-sync/internal/config"
)

var (
	withDebug   bool
	logger      = log.New(os.Stdout, "[inventory-wg-sync] ", 0)
	domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$`)
)

func main() {
	path, err := xdg.SearchConfigFile("inventory-wg-sync.yml")
	if err != nil {
		logger.Fatal("cannot find the inventory-wg-sync.yml config file: ", err, ", ensure it is in $XDG_CONFIG_DIRS or $XDG_CONFIG_HOME of the root(!) user")
	}
	if !isRoot() {
		logger.Println("WARNING: not running as root, profile updates will fail")
	}

	cfg, err := config.Read(path)
	if err != nil {
		logger.Fatal("cannot read the ", path, " config file:", err)
	}
	withDebug = cfg.Debug
	allowedIPs := getAllowedIPs(cfg)
	logger.Println("discovered", len(allowedIPs), "allowed IPs")
	if len(allowedIPs) == 0 {
		logger.Println("WARNING: no allowed IPs found")
		return
	}
	if err := handleWireGuard(cfg, allowedIPs, cfg.PostUp, cfg.PostDown); err != nil {
		logger.Println("ERROR: cannot update WireGuard profile:", err)
	}
}

// getAllowedIPs returns a list of allowed IPs from the config file and inventory files
//
//nolint:gocognit // TODO: refactor
func getAllowedIPs(cfg *config.Config) []string {
	allowedIPs, excludedIPs := getConfigIPs(cfg)
	for _, invPath := range cfg.InventoryPaths {
		inv, err := ansible.NewHostsFile(invPath, &ansible.Host{})
		if err != nil {
			logger.Println("ERROR: cannot read inventory file", invPath, ":", err)
			continue
		}
		if inv == nil || len(inv.Hosts) == 0 {
			debug("inventory", invPath, "is empty")
			continue
		}
		for _, host := range inv.Hosts {
			cidrs := determineCIDRs(host.Host)
			if len(cidrs) == 0 {
				debug("host", host.Host, "is not an IP address")
				continue
			}
			for _, cidr := range cidrs {
				if !excludedIPs[cidr] {
					allowedIPs = append(allowedIPs, cidr)
				}
			}
		}
	}
	allowedIPs = kit.Uniq(allowedIPs)
	sortIPs(allowedIPs)
	return allowedIPs
}

// getConfigIPs returns a list of allowed IPs and a map of excluded IPs from the config file
func getConfigIPs(cfg *config.Config) (allowedIPs []string, excludedIPs map[string]bool) {
	allowedIPs = []string{}
	excludedIPs = map[string]bool{}
	for _, ip := range cfg.ExcludedIPs {
		cidrs := determineCIDRs(ip)
		if len(cidrs) == 0 {
			debug("excluded IP", ip, "is not an IP address")
			continue
		}
		for _, cidr := range cidrs {
			excludedIPs[cidr] = true
		}
	}

	for _, ip := range cfg.AllowedIPs {
		cidrs := determineCIDRs(ip)
		if len(cidrs) == 0 {
			debug("allowed IP", ip, "is not an IP address")
			continue
		}
		for _, cidr := range cidrs {
			if !excludedIPs[cidr] {
				allowedIPs = append(allowedIPs, cidr)
			}
		}
	}

	return allowedIPs, excludedIPs
}

// determineCIDRs takes a host (CIDR or IPv4/IPv6 address or hostname) and determines the network CIDRs for it.
// For IP addresses, a /32 or /128 CIDR is returned depending on the address type (IPv4 or IPv6, respectively).
// For hostnames, a combination of multiple IPv4 and IPv6 CIDRs may be returned, depending on A/AAAA DNS records.
func determineCIDRs(host string) []string {
	// if CIDR, return as is
	if _, _, err := net.ParseCIDR(host); err == nil {
		return []string{host}
	}
	// if IP, return CIDR
	if ip := net.ParseIP(host); ip != nil {
		return []string{ipToCIDR(ip)}
	}
	// check if domain
	if len(host) < 4 || len(host) > 77 {
		return []string{}
	}
	if !domainRegex.MatchString(host) {
		return []string{}
	}

	// if domain with A or AAAA records, return CIDR
	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		result := []string{}
		for _, ip := range ips {
			result = append(result, ipToCIDR(ip))
		}
		return result
	}

	// if domain with CNAME record, run again
	if cname, err := net.LookupCNAME(host); err == nil && cname != "" {
		return determineCIDRs(cname)
	}

	return []string{}
}

func sortIPs(ips []string) {
	sort.Slice(ips, func(i, j int) bool {
		ipI := strings.Split(ips[i], "/")[0]
		ipJ := strings.Split(ips[j], "/")[0]
		return bytes.Compare(net.ParseIP(ipI), net.ParseIP(ipJ)) < 0
	})
}

func ipToCIDR(ip net.IP) string {
	if ip.To4() != nil {
		return ip.String() + "/32"
	}
	return ip.String() + "/128"
}

func isRoot() bool {
	return os.Geteuid() == 0
}

func debug(args ...any) {
	if !withDebug {
		return
	}
	logger.Println(args...)
}
