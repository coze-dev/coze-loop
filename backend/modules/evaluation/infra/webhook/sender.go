// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// IWebhookSender defines the interface for sending webhook notifications.
type IWebhookSender interface {
	Send(ctx context.Context, url string, payload []byte, spaceSK string, deliveryID string) (*SendResult, error)
}

// SendResult holds the result of a webhook delivery attempt.
type SendResult struct {
	StatusCode   int
	ErrorMessage string
	Success      bool
}

// NewWebhookSender creates a new WebhookSender with SSRF protection.
func NewWebhookSender() IWebhookSender {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address: %s", addr)
			}

			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolve failed: %w", err)
			}

			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("SSRF protection: access to private IP %s is denied", ip.IP.String())
				}
			}

			if len(ips) == 0 {
				return nil, fmt.Errorf("DNS resolve returned no addresses for %s", host)
			}

			// Connect using the first resolved IP
			resolvedAddr := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, resolvedAddr)
		},
	}

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &webhookSenderImpl{client: client}
}

type webhookSenderImpl struct {
	client *http.Client
}

func (s *webhookSenderImpl) Send(ctx context.Context, url string, payload []byte, spaceSK string, deliveryID string) (*SendResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return &SendResult{
			ErrorMessage: fmt.Sprintf("create request failed: %v", err),
		}, nil
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := uuid.New().String()

	// HMAC-SHA256 signature: message = timestamp + "\n" + nonce + "\n"
	signature := computeSignature(timestamp, nonce, spaceSK)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CozeLoop-Signature", signature)
	req.Header.Set("X-CozeLoop-Timestamp", timestamp)
	req.Header.Set("X-CozeLoop-Nonce", nonce)
	req.Header.Set("X-CozeLoop-Delivery-Id", deliveryID)

	resp, err := s.client.Do(req)
	if err != nil {
		return &SendResult{
			ErrorMessage: fmt.Sprintf("request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	result := &SendResult{
		StatusCode: resp.StatusCode,
		Success:    success,
	}
	if !success {
		result.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return result, nil
}

// computeSignature generates HMAC-SHA256 signature.
// message = timestamp + "\n" + nonce + "\n", key = spaceSK, hex encoded.
func computeSignature(timestamp, nonce, spaceSK string) string {
	message := timestamp + "\n" + nonce + "\n"
	mac := hmac.New(sha256.New, []byte(spaceSK))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// isPrivateIP checks if an IP address belongs to a private/reserved network.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network *net.IPNet
	}{
		{mustParseCIDR("10.0.0.0/8")},
		{mustParseCIDR("172.16.0.0/12")},
		{mustParseCIDR("192.168.0.0/16")},
		{mustParseCIDR("127.0.0.0/8")},
		{mustParseCIDR("169.254.169.254/32")},
	}

	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseCIDR(s string) *net.IPNet {
	_, network, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR: %s", s))
	}
	return network
}
