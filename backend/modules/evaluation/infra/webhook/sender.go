// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

const (
	headerTimestamp       = "X-CozeLoop-Timestamp"
	headerNonce           = "X-CozeLoop-Nonce"
	headerSignature       = "X-CozeLoop-Signature"
	headerDeliveryID      = "X-CozeLoop-Delivery-Id"
	headerFornaxTimestamp = "X-Fornax-Timestamp"
	headerFornaxNonce     = "X-Fornax-Nonce"
	headerFornaxSignature = "X-Fornax-Signature"
	headerFornaxDelivery  = "X-Fornax-Delivery-Id"
	userAgent             = "CozeLoop-Webhook/1.0"
)

type SenderImpl struct {
	client       *http.Client
	blockedCIDRs []*net.IPNet
	blockedHosts map[string]struct{}
}

func NewWebhookSender() componentwebhook.IWebhookSender {
	return NewWebhookSenderWithConf(entity.DefaultWebhookRetryConf(), entity.DefaultWebhookSecurityConf())
}

func NewWebhookSenderWithConf(retryConf *entity.WebhookRetryConf, securityConf *entity.WebhookSecurityConf) componentwebhook.IWebhookSender {
	timeout := entity.DefaultWebhookRetryConf().HTTPTimeout
	if retryConf != nil && retryConf.HTTPTimeout > 0 {
		timeout = retryConf.HTTPTimeout
	}
	sender := &SenderImpl{
		blockedCIDRs: parseCIDRs(entity.DefaultWebhookSecurityConf().BlockedCIDRs),
		blockedHosts: make(map[string]struct{}),
	}
	if securityConf != nil {
		sender.blockedCIDRs = parseCIDRs(securityConf.BlockedCIDRs)
		for _, host := range securityConf.BlockedHosts {
			sender.blockedHosts[strings.ToLower(strings.TrimSpace(host))] = struct{}{}
		}
	}
	sender.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 3 * time.Second,
			DialContext:         sender.dialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return sender
}

func (s *SenderImpl) Send(ctx context.Context, rawURL string, payload *entity.WebhookPayload, secret string) *componentwebhook.SendResult {
	if payload == nil {
		return &componentwebhook.SendResult{Error: fmt.Errorf("webhook payload is nil")}
	}
	if err := s.validateURL(ctx, rawURL); err != nil {
		return &componentwebhook.SendResult{Error: err}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return &componentwebhook.SendResult{Error: fmt.Errorf("marshal webhook payload failed: %w", err)}
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce, err := newNonce()
	if err != nil {
		return &componentwebhook.SendResult{Error: fmt.Errorf("generate nonce failed: %w", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return &componentwebhook.SendResult{Error: fmt.Errorf("create webhook request failed: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(headerTimestamp, timestamp)
	req.Header.Set(headerNonce, nonce)
	signature := computeSignature(secret, timestamp, body)
	req.Header.Set(headerSignature, signature)
	req.Header.Set(headerDeliveryID, payload.DeliveryID)
	req.Header.Set(headerFornaxTimestamp, timestamp)
	req.Header.Set(headerFornaxNonce, nonce)
	req.Header.Set(headerFornaxSignature, signature)
	req.Header.Set(headerFornaxDelivery, payload.DeliveryID)

	resp, err := s.client.Do(req)
	if err != nil {
		return &componentwebhook.SendResult{Error: err}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	return &componentwebhook.SendResult{
		Success:    resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices,
		StatusCode: resp.StatusCode,
	}
}

func (s *SenderImpl) validateURL(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || parsed.Hostname() == "" {
		return fmt.Errorf("invalid webhook url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid webhook url scheme: %s", parsed.Scheme)
	}
	host := strings.ToLower(parsed.Hostname())
	if _, ok := s.blockedHosts[host]; ok {
		return fmt.Errorf("webhook host is blocked: %s", host)
	}
	return s.ensureHostAllowed(ctx, host)
}

func (s *SenderImpl) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if err := s.ensureHostAllowed(ctx, host); err != nil {
		return nil, err
	}
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

func (s *SenderImpl) ensureHostAllowed(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if s.isBlockedIP(ip) {
			return fmt.Errorf("webhook target ip is blocked: %s", host)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve webhook host failed: %w", err)
	}
	for _, addr := range addrs {
		if s.isBlockedIP(addr.IP) {
			return fmt.Errorf("webhook target ip is blocked: %s", addr.IP.String())
		}
	}
	return nil
}

func (s *SenderImpl) isBlockedIP(ip net.IP) bool {
	for _, cidr := range s.blockedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		_, cidr, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && cidr != nil {
			result = append(result, cidr)
		}
	}
	return result
}

func newNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func computeSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
