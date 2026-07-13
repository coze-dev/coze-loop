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
	return &webhookSender{
		retry:    retry,
		security: security,
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Disable auto-follow to prevent SSRF via internal redirects.
				return http.ErrUseLastResponse
			},
		},
		secretResolver: staticSecretResolver{},
		now:            time.Now,
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
	if err := s.guardURL(delivery.URL, delivery.InternalSource); err != nil {
		return 0, err
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
func (s *webhookSender) guardURL(rawURL, internalSource string) error {
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
	if IsPrivateHost(u.Host) {
		return fmt.Errorf("private_network host rejected: %s", u.Host)
	}
	return nil
}

// IsPrivateHost returns true if the given host part (possibly host:port or
// bracketed IPv6) resolves to a private / loopback / link-local address.
// Handles: 10.x, 172.16-31.x, 192.168.x, 127.x, ::1, 169.254.x (incl. AWS
// metadata 169.254.169.254). Hostname strings that don't parse as IP are
// treated as public — the sender relies on the network layer to reject
// non-resolving hosts.
func IsPrivateHost(hostport string) bool {
	host := hostport
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		if end > 0 {
			host = host[1:end]
		}
	} else if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host, "::") {
		host = host[:i]
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		switch {
		case ip4[0] == 10:
			return true
		case ip4[0] == 127:
			return true
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return true
		case ip4[0] == 192 && ip4[1] == 168:
			return true
		case ip4[0] == 169 && ip4[1] == 254:
			return true
		}
	}
	if ip.Equal(net.IPv6loopback) {
		return true
	}
	return false
}
