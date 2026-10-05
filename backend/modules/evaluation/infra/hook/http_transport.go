// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EndpointPolicyProvider reads trusted current configuration and must honor ctx.
type EndpointPolicyProvider func(context.Context) ([]EndpointPolicy, error)

type HTTPTransport struct {
	keys     hookcomponent.KeyResolver
	policies EndpointPolicyProvider
	lookup   func(context.Context, string, string) ([]netip.Addr, error)
	dial     func(context.Context, string, string) (net.Conn, error)
	now      func() time.Time
	nonce    func() (string, error)
	rootCAs  *x509.CertPool
}

var _ hookcomponent.HTTPTransport = (*HTTPTransport)(nil)

var errHookPeerPolicy = errors.New("hook connection violates endpoint policy")

// NewHTTPTransport is lazy: missing runtime configuration fails Invoke, not startup.
func NewHTTPTransport(keys hookcomponent.KeyResolver, policies EndpointPolicyProvider) *HTTPTransport {
	if keys != nil {
		value := reflect.ValueOf(keys)
		switch value.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			if value.IsNil() {
				keys = nil
			}
		}
	}
	return &HTTPTransport{keys: keys, policies: policies, lookup: net.DefaultResolver.LookupNetIP,
		dial: (&net.Dialer{}).DialContext, now: time.Now, nonce: newHTTPNonce}
}

func newHTTPNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce[:]), nil
}

func (t *HTTPTransport) Invoke(ctx context.Context, input entity.HookTransportInput) (result entity.HookTransportResult) {
	now := time.Now
	if t != nil && t.now != nil {
		now = t.now
	}
	defer func() { result.LocalCompletedAt = now() }()
	result.Outcome = entity.ClassifyHookOutcome(entity.HookSecurityError, 0, "", false)
	if ctx == nil {
		return result
	}
	if input.Remaining <= 0 || ctx.Err() != nil {
		result.Outcome = entity.ClassifyHookOutcome(entity.HookTimeoutUncertain, 0, "", false)
		return result
	}
	config := input.Config
	if t == nil || t.keys == nil || t.policies == nil || config == nil || config.TimeoutSeconds == nil ||
		*config.TimeoutSeconds < 0 || *config.TimeoutSeconds > 1200 {
		return result
	}
	timeout := time.Duration(*config.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, min(input.Remaining, timeout))
	defer cancel()
	defer func() {
		// A non-200 body is diagnostic only; its read failure must not erase HTTP classification.
		if ctx.Err() != nil && (result.Outcome.HTTPStatus == 0 || result.Outcome.HTTPStatus == 200) {
			result.Response = nil
			result.Outcome = entity.ClassifyHookOutcome(entity.HookTimeoutUncertain, result.Outcome.HTTPStatus, "", false)
		}
	}()
	if config.Enabled == nil || !*config.Enabled || config.AccessProtocol == nil || *config.AccessProtocol != entity.HookAccessProtocolHTTP ||
		config.Environment == nil || config.InvokeHTTPInfo == nil || config.InvokeHTTPInfo.Method == nil || *config.InvokeHTTPInfo.Method != "post" ||
		config.InvokeHTTPInfo.URL == nil || input.WorkspaceID <= 0 || input.Request == nil || input.Request.Context == nil ||
		input.Request.Context.GetWorkspaceID() != strconv.FormatInt(input.WorkspaceID, 10) {
		return result
	}
	rawURL := *config.InvokeHTTPInfo.URL
	if strings.Contains(rawURL, "#") {
		return result
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return result
	}
	lane := ""
	if config.Lane != nil {
		lane = *config.Lane
	}
	policies, err := t.policies(ctx)
	if err != nil || ctx.Err() != nil {
		return result
	}
	policy, err := MatchEndpoint(EndpointTarget{WorkspaceID: input.WorkspaceID, URL: u, Environment: string(*config.Environment), Lane: lane}, policies)
	if err != nil {
		return result
	}
	rawBody, err := BuildRequest(input.Request)
	if err != nil {
		return result
	}
	key, err := t.keys.Resolve(ctx, entity.HookKeyBinding{WorkspaceID: input.WorkspaceID, URL: rawURL, Environment: *config.Environment, Lane: lane})
	if err != nil || ctx.Err() != nil {
		return result
	}
	fingerprint, err := endpointPolicyFingerprint(policy)
	if err != nil || key.PolicyFingerprint != fingerprint {
		return result
	}
	nonce, err := t.nonce()
	if err != nil {
		return result
	}
	headers, err := SignV1(SignatureInput{Method: http.MethodPost, URL: u, UnixSeconds: now().Unix(), Nonce: nonce, OperationID: input.Request.GetOperationID(), RawBody: rawBody}, key.KeyID, key.Secret)
	if err != nil {
		return result
	}
	var addresses []netip.Addr
	if ip, err := netip.ParseAddr(policy.Host); err == nil {
		addresses = []netip.Addr{ip}
	} else {
		addresses, err = t.lookup(ctx, "ip", policy.Host)
		if err != nil {
			result.Outcome = classifyHTTPTransportError(err)
			return result
		}
	}
	if ValidateEndpointIPs(policy, addresses) != nil || ctx.Err() != nil {
		return result
	}
	addresses = append([]netip.Addr(nil), addresses...)
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true, ForceAttemptHTTP2: false,
		TLSNextProto:           map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12, ServerName: policy.Host, RootCAs: t.rootCAs, NextProtos: []string{"http/1.1"}},
		MaxResponseHeaderBytes: 64 * 1024,
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			// net/http can detach the dial context; retain this invocation's total budget.
			return t.dialChecked(ctx, policy, addresses)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(rawBody))
	if err != nil {
		return result
	}
	request.GetBody = nil
	request.Header = headers
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	if *config.Environment != entity.HookEnvironmentProd {
		request.Header.Set("x-tt-env", lane)
		if *config.Environment == entity.HookEnvironmentPPE {
			request.Header.Set("x-use-ppe", "1")
		}
	}
	response, err := client.Do(request)
	if err != nil {
		result.Outcome = classifyHTTPTransportError(err)
		return result
	}
	defer response.Body.Close()
	result.Response, result.Outcome = readHTTPResponse(response)
	if result.Outcome.Code == entity.HookHTTPError && result.Outcome.Retryable {
		result.RetryAfter = safeHTTPRetryAfter(response.Header.Get("Retry-After"))
	}
	return result
}

func (t *HTTPTransport) dialChecked(ctx context.Context, policy EndpointPolicy, addresses []netip.Addr) (net.Conn, error) {
	var lastErr error
	for _, address := range addresses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		expected := netip.AddrPortFrom(address.Unmap(), policy.Port)
		conn, err := t.dial(ctx, "tcp", expected.String())
		if err != nil {
			if conn != nil {
				_ = conn.Close()
			}
			lastErr = err
			continue
		}
		if conn == nil {
			return nil, errHookPeerPolicy
		}
		if conn.RemoteAddr() == nil {
			_ = conn.Close()
			return nil, errHookPeerPolicy
		}
		peer, err := netip.ParseAddrPort(conn.RemoteAddr().String())
		if err != nil || peer.Addr().Zone() != "" || netip.AddrPortFrom(peer.Addr().Unmap(), peer.Port()) != expected ||
			ValidateEndpointIPs(policy, []netip.Addr{peer.Addr()}) != nil {
			_ = conn.Close()
			return nil, errHookPeerPolicy
		}
		deadline, _ := ctx.Deadline()
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}
	return nil, lastErr
}

func classifyHTTPTransportError(err error) entity.HookOutcome {
	code := entity.HookTransportError
	var cert *tls.CertificateVerificationError
	if errors.Is(err, errHookPeerPolicy) || errors.As(err, &cert) {
		code = entity.HookSecurityError
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		code = entity.HookTimeoutUncertain
	} else {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			code = entity.HookTimeoutUncertain
		}
	}
	return entity.ClassifyHookOutcome(code, 0, "", false)
}

func readHTTPResponse(response *http.Response) (*spi.InvokeExperimentHookResponse, entity.HookOutcome) {
	if response.StatusCode != http.StatusOK {
		return DecodeResponse(response.StatusCode, response.Header.Get("Content-Type"), response.Body)
	}
	fail := func(code entity.HookOutcomeCode) (*spi.InvokeExperimentHookResponse, entity.HookOutcome) {
		return nil, entity.ClassifyHookOutcome(code, response.StatusCode, "", false)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32*1024+1))
	if len(raw) > 32*1024 {
		return fail(entity.HookResponseTooLarge)
	}
	if err != nil {
		outcome := classifyHTTPTransportError(err)
		outcome.HTTPStatus = response.StatusCode
		return nil, outcome
	}
	encoding := strings.ToLower(strings.TrimSpace(strings.Join(response.Header.Values("Content-Encoding"), ",")))
	switch encoding {
	case "", "identity":
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return fail(entity.HookProtocolError)
		}
		defer reader.Close()
		raw, err = io.ReadAll(io.LimitReader(reader, 32*1024+1))
		if len(raw) > 32*1024 {
			return fail(entity.HookResponseTooLarge)
		}
		if err != nil {
			return fail(entity.HookProtocolError)
		}
	default:
		return fail(entity.HookProtocolError)
	}
	return DecodeResponse(response.StatusCode, response.Header.Get("Content-Type"), bytes.NewReader(raw))
}

func safeHTTPRetryAfter(raw string) string {
	if len(raw) > 128 {
		return ""
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	digits := true
	for _, c := range value {
		if c < '0' || c > '9' {
			digits = false
			break
		}
	}
	if digits {
		seconds, err := strconv.ParseUint(value, 10, 64)
		if err != nil || seconds >= 60 {
			return "60"
		}
		return strconv.FormatUint(seconds, 10)
	}
	if date, err := http.ParseTime(value); err == nil {
		return date.UTC().Format(http.TimeFormat)
	}
	return ""
}
