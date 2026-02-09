package utils

import (
	"bytes"
	"net"
	"regexp"
	"sort"
	"strings"
)

var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$`)

// DetermineCIDRs takes a host (CIDR or IPv4/IPv6 address or hostname) and determines the network CIDRs for it.
// For IP addresses, a /32 or /128 CIDR is returned depending on the address type (IPv4 or IPv6, respectively).
// For hostnames, a combination of multiple IPv4 and IPv6 CIDRs may be returned, depending on A/AAAA DNS records.
func DetermineCIDRs(host string) []string {
	// if CIDR, return as is
	if _, _, err := net.ParseCIDR(host); err == nil {
		return []string{host}
	}
	// if IP, return CIDR
	if ip := net.ParseIP(host); ip != nil {
		return []string{ipToCIDR(ip)}
	}
	if !isDomain(host) {
		return []string{}
	}

	// if domain with A or AAAA records, return CIDR
	if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
		result := make([]string, 0, len(ips))
		for _, ip := range ips {
			result = append(result, ipToCIDR(ip))
		}
		return result
	}

	// if domain with CNAME record, run again
	if cname, err := net.LookupCNAME(host); err == nil && cname != "" {
		return DetermineCIDRs(cname)
	}

	return []string{}
}

func SortIPs(ips []string) {
	sort.Slice(ips, func(i, j int) bool {
		ipI := strings.Split(ips[i], "/")[0]
		ipJ := strings.Split(ips[j], "/")[0]
		return bytes.Compare(net.ParseIP(ipI), net.ParseIP(ipJ)) < 0
	})
}

func isDomain(host string) bool {
	if len(host) < 4 || len(host) > 77 {
		return false
	}
	return domainRegex.MatchString(host)
}

func ipToCIDR(ip net.IP) string {
	if ip.To4() != nil {
		return ip.String() + "/32"
	}
	return ip.String() + "/128"
}
