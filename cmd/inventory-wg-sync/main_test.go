package main

import (
	"reflect"
	"testing"

	"github.com/etkecc/inventory-wg-sync/internal/config"
)

func TestDetermineCIDRs_NoDNS(t *testing.T) {
	tests := []struct {
		name string
		host string
		want []string
	}{
		{name: "ipv4", host: "1.2.3.4", want: []string{"1.2.3.4/32"}},
		{name: "ipv6", host: "2001:db8::1", want: []string{"2001:db8::1/128"}},
		{name: "cidr", host: "10.0.0.0/8", want: []string{"10.0.0.0/8"}},
		{name: "invalid", host: "not_a_host", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineCIDRs(tt.host)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("determineCIDRs(%q) = %#v, want %#v", tt.host, got, tt.want)
			}
		})
	}
}

func TestSortIPs(t *testing.T) {
	ips := []string{
		"2001:db8::2/128",
		"10.0.0.2/32",
		"10.0.0.1/32",
		"2001:db8::1/128",
	}
	sortIPs(ips)
	want := []string{
		"10.0.0.1/32",
		"10.0.0.2/32",
		"2001:db8::1/128",
		"2001:db8::2/128",
	}
	if !reflect.DeepEqual(ips, want) {
		t.Fatalf("sortIPs() = %#v, want %#v", ips, want)
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

func TestFilterOutUnsupportedIPs(t *testing.T) {
	linesIPv4Only := []string{"[Interface]", "Address = 10.0.0.1/32"}
	allowed := []string{"10.0.0.1/32", "fd00::1/128"}
	got := filterOutUnsupportedIPs(linesIPv4Only, allowed)
	want := []string{"10.0.0.1/32"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterOutUnsupportedIPs(ipv4-only) = %#v, want %#v", got, want)
	}

	linesIPv6Only := []string{"[Interface]", "Address = fd00::1/128"}
	got = filterOutUnsupportedIPs(linesIPv6Only, allowed)
	want = []string{"fd00::1/128"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterOutUnsupportedIPs(ipv6-only) = %#v, want %#v", got, want)
	}

	linesBoth := []string{"[Interface]", "Address = 10.0.0.1/32, fd00::1/128"}
	got = filterOutUnsupportedIPs(linesBoth, allowed)
	if !reflect.DeepEqual(got, allowed) {
		t.Fatalf("filterOutUnsupportedIPs(both) = %#v, want %#v", got, allowed)
	}
}

func TestApplyVars(t *testing.T) {
	out, err := applyVars("name={{ .name }},table={{ .table }}", map[string]any{
		"name":  "wg0",
		"table": 1234,
	})
	if err != nil {
		t.Fatalf("applyVars() error = %v", err)
	}
	if string(out) != "name=wg0,table=1234" {
		t.Fatalf("applyVars() = %q, want %q", string(out), "name=wg0,table=1234")
	}
}

func TestGetConfigIPs(t *testing.T) {
	cfg := &config.Config{
		AllowedIPs:  []string{"1.2.3.4", "10.0.0.0/8", "bad_host"},
		ExcludedIPs: []string{"1.2.3.4", "10.0.0.0/8", "also_bad"},
	}
	allowed, excluded := getConfigIPs(cfg)
	if len(allowed) != 0 {
		t.Fatalf("getConfigIPs() allowed = %#v, want empty", allowed)
	}
	if !excluded["1.2.3.4/32"] || !excluded["10.0.0.0/8"] {
		t.Fatalf("getConfigIPs() excluded missing expected entries: %#v", excluded)
	}
}
