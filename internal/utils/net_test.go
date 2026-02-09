package utils

import (
	"reflect"
	"testing"
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
			got := DetermineCIDRs(tt.host)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("DetermineCIDRs(%q) = %#v, want %#v", tt.host, got, tt.want)
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
	SortIPs(ips)
	want := []string{
		"10.0.0.1/32",
		"10.0.0.2/32",
		"2001:db8::1/128",
		"2001:db8::2/128",
	}
	if !reflect.DeepEqual(ips, want) {
		t.Fatalf("SortIPs() = %#v, want %#v", ips, want)
	}
}

func TestIsDomain(t *testing.T) {
	if isDomain("ab") {
		t.Fatalf("isDomain() short string should be false")
	}
	if isDomain("not a domain") {
		t.Fatalf("isDomain() invalid string should be false")
	}
	if !isDomain("example.com") {
		t.Fatalf("isDomain() valid domain should be true")
	}
}
