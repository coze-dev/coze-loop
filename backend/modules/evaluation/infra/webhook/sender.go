// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// awsMetadataIP is the well-known IMDS endpoint; blocked unless the caller
// explicitly opts into internal-source bypass.
var awsMetadataIP = net.ParseIP("169.254.169.254")

// bitsBypassKey is a context key that flags a request as originating from
// BITs internal injection — those calls target on-prem private endpoints by
// design, so the SSRF guard has to allow them through.
type bitsBypassKey struct{}

func withBITsBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, bitsBypassKey{}, true)
}

func isBITsBypass(ctx context.Context) bool {
	v, _ := ctx.Value(bitsBypassKey{}).(bool)
	return v
}

// NewWebhookSenderWithConf builds a componentwebhook.IWebhookSender pinned
// to the given retry + security config. Commercial's `webhook.NewWebhookSender`
// wraps this so config providers stay commercial-side.
func NewWebhookSenderWithConf(retry *entity.WebhookRetryConf, security *entity.WebhookSecurityConf) componentwebhook.IWebhookSender {
	if retry == nil {
		retry = entity.DefaultWebhookRetryConf()
	}
	if security == nil {
		security = entity.DefaultWebhookSecurityConf()
	}
	timeout := time.Duration(retry.RequestTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	transport := &http.Transport{
		DialContext:           newGuardedDialContext(timeout),
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &webhookSender{
		retry:    retry,
		security: security,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Disable auto-follow to prevent SSRF via internal redirects.
				return http.ErrUseLastResponse
			},
		},
		secretResolver: staticSecretResolver{},
		now:            time.Now,
	}
}

// newGuardedDialContext returns a DialContext that resolves hostnames itself
// and re-checks the resolved IPs against the SSRF blocklist before dialling.
// Without it a DNS-rebind attacker could pass guardURL by returning a public
// IP for the LookupHost call in the caller and swap in a private IP by the
// time the socket is opened. When ctx carries the BITs bypass marker the
// resolved-IP check is skipped so internal callbacks work.
func newGuardedDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	base := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		bypass := isBITsBypass(ctx)
		if ip := net.ParseIP(host); ip != nil {
			if !bypass && ipIsPrivate(ip) {
				return nil, fmt.Errorf("private_network dial denied: %s", host)
			}
			return base.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no ip resolved for %s", host)
		}
		var chosen *net.IPAddr
		for i := range ips {
			if !bypass && ipIsPrivate(ips[i].IP) {
				return nil, fmt.Errorf("private_network dial denied: %s -> %s", host, ips[i].IP)
			}
			if chosen == nil {
				chosen = &ips[i]
			}
		}
		return base.DialContext(ctx, network, net.JoinHostPort(chosen.IP.String(), port))
	}
}

// SecretResolver looks up the HMAC key for a given (space, delivery) pair.
// Default impl is `staticSecretResolver` returning env value; production
// commercial can override to plug WorkspaceService.GetSecretKey.
type SecretResolver interface {
	Resolve(ctx context.Context, delivery *entity.WebhookDelivery) ([]byte, error)
}

type staticSecretResolver struct{}

func (staticSecretResolver) Resolve(_ context.Context, _ *entity.WebhookDelivery) ([]byte, error) {
	// TODO: wire WorkspaceService.GetSecretKey(space_id); this fallback lets
	// the OSS binary run unit tests without a workspace RPC dep.
	return []byte("fornax-webhook-fallback-secret"), nil
}

type webhookSender struct {
	retry          *entity.WebhookRetryConf
	security       *entity.WebhookSecurityConf
	client         *http.Client
	secretResolver SecretResolver
	now            func() time.Time
}

// Send performs a single delivery attempt. Non-2xx and transport errors are
// reflected via `statusCode`/`err` for the caller (MQ consumer) to advance
// retry state — this function never mutates the delivery row.
func (s *webhookSender) Send(ctx context.Context, delivery *entity.WebhookDelivery) (statusCode int, err error) {
	if delivery == nil {
		return 0, errors.New("nil delivery")
	}
	if err := s.guardURL(ctx, delivery.URL, delivery.InternalSource); err != nil {
		return 0, err
	}
	if delivery.InternalSource == entity.WebhookInternalSourceBITs {
		ctx = withBITsBypass(ctx)
	}

	secret, err := s.secretResolver.Resolve(ctx, delivery)
	if err != nil {
		return 0, fmt.Errorf("resolve secret: %w", err)
	}
	ts := strconv.FormatInt(s.now().Unix(), 10)
	sig, err := SignWithAlgorithm(s.security.Algorithm, secret, ts, delivery.Payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.URL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(s.security.DeliveryIDHeader, delivery.DeliveryID)
	req.Header.Set(s.security.TimestampHeader, ts)
	req.Header.Set(s.security.SignatureHeader, sig)

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhook non-2xx: %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// guardURL enforces scheme + private-network restrictions per §HTTP client.
// `internal_source=bits` bypasses the private-network guard (BITs internal
// callback URLs deliberately point at internal endpoints).
func (s *webhookSender) guardURL(ctx context.Context, rawURL, internalSource string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("empty host")
	}
	if internalSource == entity.WebhookInternalSourceBITs {
		return nil
	}
	if isPrivateHostCtx(ctx, u.Host) {
		return fmt.Errorf("private_network host rejected: %s", u.Host)
	}
	return nil
}

// stripHost extracts the plain host component from a `host[:port]` /
// `[ipv6]:port` / `[ipv6]` string.
func stripHost(hostport string) string {
	host := hostport
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end > 0 {
			return host[1:end]
		}
	}
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[:i], ":") {
		return host[:i]
	}
	return host
}

// ipIsPrivate reports whether ip belongs to any range the SSRF guard blocks:
// RFC1918 private / loopback / link-local (incl. AWS metadata 169.254.169.254)
// / IPv6 loopback / IPv6 unique-local.
func ipIsPrivate(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}
	if awsMetadataIP != nil && ip.Equal(awsMetadataIP) {
		return true
	}
	if ip.Equal(net.IPv6loopback) {
		return true
	}
	return false
}

// isPrivateHostCtx resolves hostport and reports whether the target maps to
// any private / loopback / link-local IP. Hostnames that don't resolve are
// treated as private (deny by default) so non-existent hosts can't be used
// to slip past the guard by racing with a later DNS answer.
func isPrivateHostCtx(ctx context.Context, hostport string) bool {
	host := stripHost(hostport)
	if ip := net.ParseIP(host); ip != nil {
		return ipIsPrivate(ip)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return true
	}
	for i := range ips {
		if ipIsPrivate(ips[i].IP) {
			return true
		}
	}
	return false
}

// IsPrivateHost is the ctx-less shim retained for existing callers / tests.
// It performs DNS resolution through the default resolver.
func IsPrivateHost(hostport string) bool {
	return isPrivateHostCtx(context.Background(), hostport)
}
