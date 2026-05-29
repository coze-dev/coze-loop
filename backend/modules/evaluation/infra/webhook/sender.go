// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	defaultTimeout = 5 * time.Second
)

// SendResult Webhook 发送结果
type SendResult struct {
	StatusCode   int
	ResponseBody string
	Err          error
}

// IsSuccess 判断是否发送成功（2xx）
func (r *SendResult) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// Sender Webhook HTTP 发送器
type Sender struct {
	client *http.Client
}

// NewSenderWithClient 创建指定 HTTP client 的发送器（用于测试或自定义场景）
func NewSenderWithClient(client *http.Client) *Sender {
	return &Sender{client: client}
}

// NewSender 创建 Webhook 发送器（内置 SSRF 防护）
func NewSender() *Sender {
	transport := &http.Transport{
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("webhook target %s resolves to private IP %s, blocked", host, ip.IP)
				}
			}
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	return &Sender{
		client: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// isPrivateIP 判断是否为私有/保留 IP 地址
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		start net.IP
		end   net.IP
	}{
		{net.ParseIP("10.0.0.0"), net.ParseIP("10.255.255.255")},
		{net.ParseIP("172.16.0.0"), net.ParseIP("172.31.255.255")},
		{net.ParseIP("192.168.0.0"), net.ParseIP("192.168.255.255")},
		{net.ParseIP("127.0.0.0"), net.ParseIP("127.255.255.255")},
		{net.ParseIP("169.254.0.0"), net.ParseIP("169.254.255.255")},
	}
	for _, r := range privateRanges {
		if bytesCompare(ip, r.start) >= 0 && bytesCompare(ip, r.end) <= 0 {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func bytesCompare(a, b net.IP) int {
	a = a.To16()
	b = b.To16()
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// Send 发送 Webhook 请求
func (s *Sender) Send(ctx context.Context, url string, body []byte, secret string) *SendResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &SendResult{Err: fmt.Errorf("create request failed: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")

	// 签名（secret 为空时跳过，避免空 key HMAC）
	if secret != "" {
		timestamp, nonce, signature := SignRequest(secret)
		req.Header.Set(HeaderTimestamp, timestamp)
		req.Header.Set(HeaderNonce, nonce)
		req.Header.Set(HeaderSignature, signature)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return &SendResult{Err: fmt.Errorf("send request failed: %w", err)}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	return &SendResult{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(respBody),
	}
}
