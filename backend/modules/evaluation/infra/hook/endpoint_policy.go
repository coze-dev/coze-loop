// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

// EndpointTarget uses the business target environment, never platform execution_scope.
type EndpointTarget struct {
	WorkspaceID int64
	URL         *url.URL
	Environment string
	Lane        string
}

// EndpointPolicy is trusted operator configuration for one HTTPS host/port and
// workspace/environment/lane. Host is a bare DNS name or unbracketed IP, not a URL.
type EndpointPolicy struct {
	WorkspaceID     int64
	Host            string
	Port            uint16
	Environment     string
	Lane            string
	PrivateCIDRs    []netip.Prefix
	AllowPrivateIPs bool `json:",omitempty"`
}

// MatchEndpoint performs no DNS or network I/O and returns an owned normalized copy.
func MatchEndpoint(target EndpointTarget, allowed []EndpointPolicy) (EndpointPolicy, error) {
	if target.WorkspaceID <= 0 || !validEndpointEnvironment(target.Environment, target.Lane) {
		return EndpointPolicy{}, errors.New("invalid hook endpoint binding")
	}
	host, port, err := normalizedEndpointURL(target.URL)
	if err != nil {
		return EndpointPolicy{}, err
	}
	var matched *EndpointPolicy
	for _, entry := range allowed {
		policy, err := normalizeEndpointPolicy(entry)
		if err != nil {
			return EndpointPolicy{}, err
		}
		if policy.WorkspaceID == target.WorkspaceID && policy.Host == host && policy.Port == port &&
			policy.Environment == target.Environment && policy.Lane == target.Lane {
			if matched != nil {
				return EndpointPolicy{}, errors.New("ambiguous hook endpoint policy")
			}
			matched = &policy
		}
	}
	if matched == nil {
		return EndpointPolicy{}, errors.New("hook endpoint is not allowed")
	}
	return *matched, nil
}

// ValidateEndpointIPs checks the entire resolution against a matched trusted policy.
// It does not prevent DNS rebinding: transport must pin dialing to checked addresses
// and recheck the actual peer, including on each new connection.
func ValidateEndpointIPs(policy EndpointPolicy, addresses []netip.Addr) error {
	policy, err := normalizeEndpointPolicy(policy)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return errors.New("hook endpoint has no resolved addresses")
	}
	literal, literalErr := netip.ParseAddr(policy.Host)
	for _, address := range addresses {
		if !address.IsValid() || address.Zone() != "" {
			return errors.New("invalid hook endpoint IP")
		}
		address = address.Unmap()
		if permanentlyDeniedIP(address) {
			return errors.New("hook endpoint IP is forbidden")
		}
		if literalErr == nil && address != literal {
			return errors.New("hook endpoint IP differs from literal host")
		}
		if privateEndpointIP(address) && !policy.AllowPrivateIPs && !inEndpointCIDRs(address, policy.PrivateCIDRs) {
			return errors.New("hook endpoint private IP is not authorized")
		}
	}
	return nil
}

func validEndpointEnvironment(environment, lane string) bool {
	switch environment {
	case "Prod":
		return lane == ""
	case "PPE", "BOE":
		return signatureToken(lane)
	default:
		return false
	}
}

func normalizeEndpointPolicy(policy EndpointPolicy) (EndpointPolicy, error) {
	invalid := errors.New("invalid hook endpoint policy")
	if policy.WorkspaceID <= 0 || policy.Port == 0 || !validEndpointEnvironment(policy.Environment, policy.Lane) {
		return EndpointPolicy{}, invalid
	}
	host, err := normalizedEndpointHost(policy.Host)
	if err != nil {
		return EndpointPolicy{}, invalid
	}
	for _, prefix := range policy.PrivateCIDRs {
		if !prefix.IsValid() || prefix != prefix.Masked() || prefix.Addr().Is4In6() || !privateEndpointCIDR(prefix) {
			return EndpointPolicy{}, invalid
		}
	}
	policy.Host = host
	policy.PrivateCIDRs = append([]netip.Prefix(nil), policy.PrivateCIDRs...)
	return policy, nil
}

func normalizedEndpointURL(u *url.URL) (string, uint16, error) {
	invalid := errors.New("invalid hook HTTPS endpoint URL")
	if u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" ||
		u.Fragment != "" || u.RawFragment != "" || u.OmitHost || !utf8.ValidString(u.Path) ||
		(u.Path != "" && !strings.HasPrefix(u.Path, "/")) || strings.ContainsAny(u.Path, "\r\n\x00") {
		return "", 0, invalid
	}
	// Parse may retain a plain Unicode path as RawPath; only that lossless fallback
	// is allowed. Inconsistent or mixed invalid encodings must not be silently rewritten.
	if u.RawPath != "" && u.EscapedPath() != u.RawPath && (u.RawPath != u.Path || strings.Contains(u.RawPath, "%")) {
		return "", 0, invalid
	}
	if len(u.String()) > 2048 || !utf8.ValidString(u.String()) || strings.ContainsAny(u.RawQuery, "#\r\n") {
		return "", 0, invalid
	}
	for _, c := range u.RawQuery {
		if c <= ' ' || c == 0x7f {
			return "", 0, invalid
		}
	}
	if _, err := url.QueryUnescape(u.RawQuery); err != nil {
		return "", 0, invalid
	}
	parsed, err := url.Parse("https://" + u.Host)
	if err != nil || parsed.Host != u.Host || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || strings.HasSuffix(u.Host, ":") {
		return "", 0, invalid
	}
	if strings.HasPrefix(u.Host, "[") {
		ip, err := netip.ParseAddr(parsed.Hostname())
		if err != nil || !ip.Is6() || ip.Zone() != "" {
			return "", 0, invalid
		}
	} else if strings.Contains(parsed.Hostname(), ":") {
		return "", 0, invalid
	}
	port := uint64(443)
	if rawPort := parsed.Port(); rawPort != "" {
		port, err = strconv.ParseUint(rawPort, 10, 16)
		if err != nil || port == 0 {
			return "", 0, invalid
		}
	}
	host, err := normalizedEndpointHost(parsed.Hostname())
	if err != nil {
		return "", 0, invalid
	}
	return host, uint16(port), nil
}

// Lookup IDNA, case folding and one optional terminal dot affect matching only.
func normalizedEndpointHost(host string) (string, error) {
	invalid := errors.New("invalid hook endpoint host")
	if host == "" || !utf8.ValidString(host) || strings.ContainsAny(host, "*%/\\@?#[]") {
		return "", invalid
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String(), nil
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", invalid
	}
	ascii = strings.TrimSuffix(strings.ToLower(ascii), ".")
	if len(ascii) == 0 || len(ascii) > 253 {
		return "", invalid
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", invalid
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				return "", invalid
			}
		}
	}
	return ascii, nil
}

var endpointPrivateRanges = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("fc00::/7"),
}

func privateEndpointCIDR(prefix netip.Prefix) bool {
	for _, allowed := range endpointPrivateRanges {
		if prefix.Bits() >= allowed.Bits() && allowed.Contains(prefix.Addr()) {
			return true
		}
	}
	return false
}

func privateEndpointIP(address netip.Addr) bool {
	return inEndpointCIDRs(address, endpointPrivateRanges)
}

func inEndpointCIDRs(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func permanentlyDeniedIP(address netip.Addr) bool {
	return !address.IsGlobalUnicast() || address.IsLoopback() || address.IsLinkLocalUnicast() ||
		netip.MustParsePrefix("0.0.0.0/8").Contains(address) ||
		netip.MustParsePrefix("240.0.0.0/4").Contains(address) || netip.MustParsePrefix("fec0::/10").Contains(address) ||
		address == netip.MustParseAddr("100.100.100.200") || address == netip.MustParseAddr("fd00:ec2::254") ||
		address == netip.MustParseAddr("168.63.129.16")
}
