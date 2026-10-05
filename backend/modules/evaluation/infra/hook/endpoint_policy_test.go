// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func endpointFixture(t *testing.T) (EndpointTarget, EndpointPolicy) {
	t.Helper()
	return EndpointTarget{WorkspaceID: 123, URL: signatureURL(t, "https://HOOK.example.:443/a%2fb?z=2&a=1"), Environment: "PPE", Lane: "ppe_hook"},
		EndpointPolicy{WorkspaceID: 123, Host: "hook.example", Port: 443, Environment: "PPE", Lane: "ppe_hook"}
}

func TestMatchEndpointNormalizesOnlyAuthority(t *testing.T) {
	for _, tt := range []struct {
		raw, host, want string
		port            uint16
	}{
		{"https://HOOK.example./a%2fb?z=2&a=1", "hook.example", "hook.example", 443},
		{"https://bücher.example:443/中?", "XN--BCHER-KVA.EXAMPLE.", "xn--bcher-kva.example", 443},
		{"https://xn--bcher-kva.example:8443/%e4%b8%ad?", "bücher.example", "xn--bcher-kva.example", 8443},
		{"https://hook。example/", "HOOK.example", "hook.example", 443},
		{"https://[2001:4860:4860::8888]/", "2001:4860:4860:0:0:0:0:8888", "2001:4860:4860::8888", 443},
		{"https://[::ffff:8.8.8.8]/", "8.8.8.8", "8.8.8.8", 443},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			target, policy := endpointFixture(t)
			target.URL, policy.Host, policy.Port = signatureURL(t, tt.raw), tt.host, tt.port
			policy.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("10.2.3.0/24")}
			before := *target.URL
			matched, err := MatchEndpoint(target, []EndpointPolicy{policy})
			if err != nil || matched.Host != tt.want || matched.Port != tt.port {
				t.Fatalf("matched=%+v err=%v", matched, err)
			}
			if !reflect.DeepEqual(before, *target.URL) {
				t.Fatal("host normalization changed signature URL bytes")
			}
			matched.PrivateCIDRs[0] = netip.MustParsePrefix("10.8.0.0/16")
			if policy.PrivateCIDRs[0].String() != "10.2.3.0/24" {
				t.Fatal("matched policy aliases trusted configuration")
			}
		})
	}
}

func TestMatchEndpointRejectsCrossBinding(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*EndpointTarget)
	}{
		{"workspace", func(v *EndpointTarget) { v.WorkspaceID++ }},
		{"host", func(v *EndpointTarget) { v.URL.Host = "evil.example" }},
		{"host suffix", func(v *EndpointTarget) { v.URL.Host = "hook.example.evil" }},
		{"port", func(v *EndpointTarget) { v.URL.Host = "hook.example:8443" }},
		{"environment", func(v *EndpointTarget) { v.Environment = "BOE" }},
		{"lane", func(v *EndpointTarget) { v.Lane = "ppe_other" }},
		{"platform scope is not target", func(v *EndpointTarget) { v.Environment = "prod"; v.Lane = "" }},
		{"Prod with lane", func(v *EndpointTarget) { v.Environment = "Prod" }},
		{"empty lane", func(v *EndpointTarget) { v.Lane = "" }},
		{"lane injection", func(v *EndpointTarget) { v.Lane = "ppe_hook\r\nX: y" }},
		{"nonpositive workspace", func(v *EndpointTarget) { v.WorkspaceID = 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target, policy := endpointFixture(t)
			tt.change(&target)
			if _, err := MatchEndpoint(target, []EndpointPolicy{policy}); err == nil {
				t.Fatal("cross-binding or invalid target admitted")
			}
		})
	}
	for _, environment := range []string{"Prod", "PPE", "BOE"} {
		target, policy := endpointFixture(t)
		target.Environment, policy.Environment = environment, environment
		if environment == "Prod" {
			target.Lane, policy.Lane = "", ""
		}
		if _, err := MatchEndpoint(target, []EndpointPolicy{policy}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMatchEndpointRejectsMalformedURLs(t *testing.T) {
	const secret = "PRIVATE-URL-MATERIAL"
	for _, tt := range []struct {
		name   string
		change func(*url.URL)
	}{
		{"http", func(u *url.URL) { u.Scheme = "http" }},
		{"empty host", func(u *url.URL) { u.Host = "" }},
		{"userinfo", func(u *url.URL) { u.User = url.UserPassword("u", secret) }},
		{"fragment", func(u *url.URL) { u.Fragment = secret }},
		{"raw fragment", func(u *url.URL) { u.RawFragment = secret }},
		{"opaque", func(u *url.URL) { u.Opaque = secret }},
		{"empty port", func(u *url.URL) { u.Host = "hook.example:" }},
		{"zero port", func(u *url.URL) { u.Host = "hook.example:0" }},
		{"overflow port", func(u *url.URL) { u.Host = "hook.example:65536" }},
		{"negative port", func(u *url.URL) { u.Host = "hook.example:-1" }},
		{"text port", func(u *url.URL) { u.Host = "hook.example:https" }},
		{"zone", func(u *url.URL) { u.Host = "[fe80::1%en0]" }},
		{"unbracketed IPv6", func(u *url.URL) { u.Host = "2001:4860:4860::8888" }},
		{"bracketed hostname", func(u *url.URL) { u.Host = "[hook.example]" }},
		{"wildcard", func(u *url.URL) { u.Host = "*.example" }},
		{"empty label", func(u *url.URL) { u.Host = "hook..example" }},
		{"extra terminal dot", func(u *url.URL) { u.Host = "hook.example.." }},
		{"underscore", func(u *url.URL) { u.Host = "hook_example" }},
		{"leading hyphen", func(u *url.URL) { u.Host = "-hook.example" }},
		{"relative path", func(u *url.URL) { u.Path = "hook"; u.RawPath = "" }},
		{"oversize", func(u *url.URL) { u.Path = "/" + strings.Repeat("x", 2048); u.RawPath = "" }},
		{"bad utf8", func(u *url.URL) { u.Host = "\xff.example" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target, policy := endpointFixture(t)
			tt.change(target.URL)
			_, err := MatchEndpoint(target, []EndpointPolicy{policy})
			if err == nil || strings.Contains(err.Error(), secret) {
				t.Fatal("malformed URL admitted or sensitive input leaked")
			}
		})
	}
}

func TestMatchEndpointFailsClosedOnInvalidPolicies(t *testing.T) {
	target, policy := endpointFixture(t)
	if _, err := MatchEndpoint(target, nil); err == nil {
		t.Fatal("empty policy admitted")
	}
	for _, tt := range []struct {
		name   string
		change func(*EndpointPolicy)
	}{
		{"wildcard host", func(v *EndpointPolicy) { v.Host = "*.example" }},
		{"wildcard workspace", func(v *EndpointPolicy) { v.WorkspaceID = 0 }},
		{"wildcard port", func(v *EndpointPolicy) { v.Port = 0 }},
		{"unknown environment", func(v *EndpointPolicy) { v.Environment = "*" }},
		{"lane injection", func(v *EndpointPolicy) { v.Lane = "ppe_hook\n" }},
		{"empty host", func(v *EndpointPolicy) { v.Host = "" }},
		{"private override all IPv4", func(v *EndpointPolicy) { v.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")} }},
		{"private override all IPv6", func(v *EndpointPolicy) { v.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("::/0")} }},
		{"public CIDR", func(v *EndpointPolicy) { v.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("8.8.8.0/24")} }},
		{"invalid CIDR", func(v *EndpointPolicy) { v.PrivateCIDRs = []netip.Prefix{{}} }},
		{"unmasked CIDR", func(v *EndpointPolicy) { v.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")} }},
		{"mapped CIDR", func(v *EndpointPolicy) { v.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix("::ffff:10.0.0.0/104")} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bad := policy
			tt.change(&bad)
			if _, err := MatchEndpoint(target, []EndpointPolicy{bad}); err == nil {
				t.Fatal("invalid policy admitted")
			}
		})
	}
}

func TestValidateEndpointIPsRequiresEveryAddress(t *testing.T) {
	_, policy := endpointFixture(t)
	for _, tt := range []struct {
		name      string
		addresses []netip.Addr
		wantErr   bool
	}{
		{"empty", nil, true},
		{"invalid", []netip.Addr{{}}, true},
		{"public dual stack", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("2606:4700:4700::1111")}, false},
		{"mapped public", []netip.Addr{netip.MustParseAddr("::ffff:8.8.8.8")}, false},
		{"public then loopback", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, true},
		{"loopback then public", []netip.Addr{netip.MustParseAddr("::1"), netip.MustParseAddr("8.8.8.8")}, true},
		{"public then private", []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.2.3.4")}, true},
		{"zone", []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111%en0")}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]netip.Addr(nil), tt.addresses...)
			err := ValidateEndpointIPs(policy, tt.addresses)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(before, tt.addresses) {
				t.Fatal("addresses mutated")
			}
		})
	}
}

func TestValidateEndpointIPsPrivateCIDRsAreExplicitAndNarrow(t *testing.T) {
	for _, tt := range []struct{ cidr, inside, outside string }{
		{"10.2.3.0/24", "10.2.3.4", "10.2.4.1"},
		{"172.20.1.0/24", "172.20.1.2", "172.20.2.2"},
		{"192.168.1.0/24", "192.168.1.2", "192.168.2.2"},
		{"100.64.1.0/24", "100.64.1.2", "100.65.1.2"},
		{"fd12:3456::/48", "fd12:3456::1", "fd12:3457::1"},
	} {
		t.Run(tt.cidr, func(t *testing.T) {
			_, policy := endpointFixture(t)
			inside := []netip.Addr{netip.MustParseAddr(tt.inside)}
			if err := ValidateEndpointIPs(policy, inside); err == nil {
				t.Fatal("private address admitted by default")
			}
			policy.PrivateCIDRs = []netip.Prefix{netip.MustParsePrefix(tt.cidr)}
			if err := ValidateEndpointIPs(policy, inside); err != nil {
				t.Fatal(err)
			}
			if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr(tt.outside)}); err == nil {
				t.Fatal("CIDR granted outside range")
			}
			if inside[0].Is4() {
				if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr("::ffff:" + tt.inside)}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestValidateEndpointIPsPermanentDenials(t *testing.T) {
	_, policy := endpointFixture(t)
	policy.PrivateCIDRs = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("fc00::/7"),
	}
	for _, raw := range []string{
		"127.0.0.1", "127.255.255.254", "::1", "0.0.0.0", "::", "0.1.2.3",
		"169.254.1.1", "169.254.169.254", "fe80::1", "ff02::1", "224.0.0.1", "255.255.255.255",
		"100.100.100.200", "fd00:ec2::254", "::ffff:127.0.0.1", "::ffff:100.100.100.200",
		"::ffff:169.254.169.254", "::ffff:0.0.0.0", "::ffff:224.0.0.1",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr(raw)}); err == nil {
				t.Fatal("permanently forbidden IP admitted")
			}
		})
	}
}

func TestValidateEndpointIPsBindsLiteralHost(t *testing.T) {
	_, policy := endpointFixture(t)
	policy.Host = "8.8.8.8"
	if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr("8.8.8.8")}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr("1.1.1.1")}); err == nil {
		t.Fatal("literal host address substituted")
	}
	policy.Host = "127.0.0.1"
	if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr("127.0.0.1")}); err == nil {
		t.Fatal("literal host bypassed IP policy")
	}
}

func TestMatchEndpointDuplicatePoliciesFailClosed(t *testing.T) {
	target, policy := endpointFixture(t)
	if _, err := MatchEndpoint(target, []EndpointPolicy{policy, policy}); err == nil {
		t.Fatal("ambiguous policies admitted")
	}
	other := policy
	other.WorkspaceID++
	if matched, err := MatchEndpoint(target, []EndpointPolicy{other, policy}); err != nil || matched.WorkspaceID != 123 {
		t.Fatal("exact policy not selected")
	}
}

func TestMatchEndpointURLByteBoundary(t *testing.T) {
	target, policy := endpointFixture(t)
	target.URL = signatureURL(t, "https://hook.example/"+strings.Repeat("x", 2048-len("https://hook.example/")))
	if _, err := MatchEndpoint(target, []EndpointPolicy{policy}); err != nil {
		t.Fatal(err)
	}
	target.URL.Path += "x"
	if _, err := MatchEndpoint(target, []EndpointPolicy{policy}); err == nil {
		t.Fatal("oversize URL admitted")
	}
}

func TestValidateEndpointIPsRejectsPlatformSpecialAddresses(t *testing.T) {
	_, policy := endpointFixture(t)
	for _, raw := range []string{"168.63.129.16", "::ffff:168.63.129.16", "240.0.0.1", "fec0::1"} {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateEndpointIPs(policy, []netip.Addr{netip.MustParseAddr(raw)}); err == nil {
				t.Fatal("platform or reserved address admitted")
			}
		})
	}
}
