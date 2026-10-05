// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type httpTestKeys func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error)

func (f httpTestKeys) Resolve(ctx context.Context, b entity.HookKeyBinding) (entity.HookSigningKey, error) {
	return f(ctx, b)
}

func httpTestInput() entity.HookTransportInput {
	return entity.HookTransportInput{
		WorkspaceID: 1, Remaining: 2 * time.Second, Request: validRequest(),
		Config: &entity.HookConfig{Enabled: gptr.Of(true), AccessProtocol: gptr.Of(entity.HookAccessProtocolHTTP), Environment: gptr.Of(entity.HookEnvironmentProd), TimeoutSeconds: gptr.Of(int32(180)), InvokeHTTPInfo: &entity.HookHTTPInfo{Method: gptr.Of("post"), URL: gptr.Of("https://example.com/hook?token=private-query")}},
	}
}

func httpTestPolicy() EndpointPolicy {
	return EndpointPolicy{WorkspaceID: 1, Host: "example.com", Port: 443, Environment: "Prod"}
}

func httpTestSigningKey(policy EndpointPolicy, keyID string) (entity.HookSigningKey, error) {
	fingerprint, err := endpointPolicyFingerprint(policy)
	return entity.HookSigningKey{KeyID: keyID, Secret: []byte("private-test-secret"), PolicyFingerprint: fingerprint}, err
}

func httpTestTransport() *HTTPTransport {
	tr := NewHTTPTransport(httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
		return httpTestSigningKey(httpTestPolicy(), "test-key")
	}), func(context.Context) ([]EndpointPolicy, error) { return []EndpointPolicy{httpTestPolicy()}, nil })
	tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("203.0.113.10")}, nil
	}
	tr.dial = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("private-test-secret private-query dial error")
	}
	return tr
}

func assertHTTPOutcome(t *testing.T, result entity.HookTransportResult, code entity.HookOutcomeCode) {
	t.Helper()
	if result.Outcome.Code != code {
		t.Fatalf("outcome=%+v want=%s", result.Outcome, code)
	}
	if result.LocalCompletedAt.IsZero() {
		t.Fatal("missing local completion anchor")
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-test-secret", "private-query", "private-response", "resolver-private"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("sensitive detail leaked into visible result")
		}
	}
}

func TestHookHTTPMissingDependenciesAreLazyAndFailClosed(t *testing.T) {
	var calls atomic.Int32
	keys := httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
		calls.Add(1)
		return entity.HookSigningKey{}, nil
	})
	policies := func(context.Context) ([]EndpointPolicy, error) { calls.Add(1); return nil, nil }
	unused := NewHTTPTransport(keys, policies)
	if unused == nil || calls.Load() != 0 {
		t.Fatal("constructor eagerly required runtime config")
	}
	for _, tr := range []*HTTPTransport{NewHTTPTransport(nil, policies), NewHTTPTransport(keys, nil), unused, nil} {
		assertHTTPOutcome(t, tr.Invoke(context.Background(), httpTestInput()), entity.HookSecurityError)
	}
}

func TestHookHTTPInvalidInputsNeverDial(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*entity.HookTransportInput)
	}{
		{"nil config", func(in *entity.HookTransportInput) { in.Config = nil }},
		{"disabled", func(in *entity.HookTransportInput) { in.Config.Enabled = gptr.Of(false) }},
		{"rpc", func(in *entity.HookTransportInput) { in.Config.AccessProtocol = gptr.Of(entity.HookAccessProtocolRPC) }},
		{"missing url", func(in *entity.HookTransportInput) { in.Config.InvokeHTTPInfo = nil }},
		{"method", func(in *entity.HookTransportInput) { in.Config.InvokeHTTPInfo.Method = gptr.Of("get") }},
		{"userinfo", func(in *entity.HookTransportInput) {
			in.Config.InvokeHTTPInfo.URL = gptr.Of("https://u:private-test-secret@example.com/")
		}},
		{"empty fragment", func(in *entity.HookTransportInput) { in.Config.InvokeHTTPInfo.URL = gptr.Of("https://example.com/#") }},
		{"scope mismatch", func(in *entity.HookTransportInput) { in.WorkspaceID = 2 }},
		{"nil request", func(in *entity.HookTransportInput) { in.Request = nil }},
		{"bad timeout", func(in *entity.HookTransportInput) { in.Config.TimeoutSeconds = gptr.Of(int32(1201)) }},
		{"PPE without lane", func(in *entity.HookTransportInput) { in.Config.Environment = gptr.Of(entity.HookEnvironmentPPE) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, tr := httpTestInput(), httpTestTransport()
			tc.change(&in)
			var dials atomic.Int32
			tr.dial = func(context.Context, string, string) (net.Conn, error) {
				dials.Add(1)
				return nil, errors.New("must not dial")
			}
			assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSecurityError)
			if dials.Load() != 0 {
				t.Fatal("invalid input reached dial")
			}
		})
	}
}

func TestHookHTTPPolicyAndKeyFailuresNeverDial(t *testing.T) {
	for _, mode := range []string{"policy error", "empty policies", "missing binding", "key error", "empty key", "empty key id", "empty fingerprint", "wrong fingerprint", "invalid nonce"} {
		t.Run(mode, func(t *testing.T) {
			tr := httpTestTransport()
			var dials atomic.Int32
			tr.dial = func(context.Context, string, string) (net.Conn, error) {
				dials.Add(1)
				return nil, errors.New("must not dial")
			}
			switch mode {
			case "policy error":
				tr.policies = func(context.Context) ([]EndpointPolicy, error) { return nil, errors.New("resolver-private") }
			case "empty policies":
				tr.policies = func(context.Context) ([]EndpointPolicy, error) { return nil, nil }
			case "missing binding":
				tr.policies = func(context.Context) ([]EndpointPolicy, error) {
					p := httpTestPolicy()
					p.WorkspaceID++
					return []EndpointPolicy{p}, nil
				}
			case "key error":
				tr.keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
					return entity.HookSigningKey{}, errors.New("resolver-private")
				})
			case "empty key":
				tr.keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
					key, err := httpTestSigningKey(httpTestPolicy(), "key")
					key.Secret = nil
					return key, err
				})
			case "empty key id":
				tr.keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
					return httpTestSigningKey(httpTestPolicy(), "")
				})
			case "empty fingerprint", "wrong fingerprint":
				tr.keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
					key, err := httpTestSigningKey(httpTestPolicy(), "key")
					key.PolicyFingerprint = ""
					if mode == "wrong fingerprint" {
						key.PolicyFingerprint = strings.Repeat("0", 64)
					}
					return key, err
				})
			case "invalid nonce":
				tr.nonce = func() (string, error) { return "", errors.New("private-test-secret") }
			}
			assertHTTPOutcome(t, tr.Invoke(context.Background(), httpTestInput()), entity.HookSecurityError)
			if dials.Load() != 0 {
				t.Fatal("unsafe dependency reached dial")
			}
		})
	}
}

func TestHookHTTPAllDNSAddressesCheckedBeforeDial(t *testing.T) {
	for _, bad := range []string{"127.0.0.1", "169.254.169.254", "::ffff:100.100.100.200", "10.0.0.1"} {
		t.Run(bad, func(t *testing.T) {
			tr := httpTestTransport()
			tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("203.0.113.10"), netip.MustParseAddr(bad)}, nil
			}
			var dials atomic.Int32
			tr.dial = func(context.Context, string, string) (net.Conn, error) { dials.Add(1); return nil, nil }
			assertHTTPOutcome(t, tr.Invoke(context.Background(), httpTestInput()), entity.HookSecurityError)
			if dials.Load() != 0 {
				t.Fatal("selected safe DNS subset instead of rejecting resolution")
			}
		})
	}
}

func TestHookHTTPDeadlineCoversDependencies(t *testing.T) {
	for _, stage := range []string{"policy", "key", "dns", "tcp"} {
		t.Run(stage, func(t *testing.T) {
			tr, in := httpTestTransport(), httpTestInput()
			in.Remaining = 40 * time.Millisecond
			wait := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
			switch stage {
			case "policy":
				tr.policies = func(ctx context.Context) ([]EndpointPolicy, error) { return nil, wait(ctx) }
			case "key":
				tr.keys = httpTestKeys(func(ctx context.Context, _ entity.HookKeyBinding) (entity.HookSigningKey, error) {
					return entity.HookSigningKey{}, wait(ctx)
				})
			case "dns":
				tr.lookup = func(ctx context.Context, _, _ string) ([]netip.Addr, error) { return nil, wait(ctx) }
			case "tcp":
				tr.dial = func(ctx context.Context, _, _ string) (net.Conn, error) { return nil, wait(ctx) }
			}
			start := time.Now()
			got := tr.Invoke(context.Background(), in)
			assertHTTPOutcome(t, got, entity.HookTimeoutUncertain)
			if time.Since(start) > time.Second || !got.Outcome.Retryable {
				t.Fatal("deadline did not bound dependency")
			}
		})
	}
}

func TestHookHTTPRemainingAndParentCancellation(t *testing.T) {
	for _, budget := range []time.Duration{0, -time.Second} {
		in := httpTestInput()
		in.Remaining = budget
		assertHTTPOutcome(t, httpTestTransport().Invoke(context.Background(), in), entity.HookTimeoutUncertain)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assertHTTPOutcome(t, httpTestTransport().Invoke(ctx, httpTestInput()), entity.HookTimeoutUncertain)
	tr, in := httpTestTransport(), httpTestInput()
	in.Remaining, in.Config.TimeoutSeconds = time.Hour, gptr.Of(int32(1))
	tr.policies = func(ctx context.Context) ([]EndpointPolicy, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Error("configuration timeout did not constrain Remaining")
		}
		return nil, nil
	}
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSecurityError)
}

func TestHookHTTPSecretsAreNotJSON(t *testing.T) {
	encoded, err := json.Marshal(entity.HookSigningKey{KeyID: "key", Secret: []byte("private-test-secret"), PolicyFingerprint: "private-policy-fingerprint"})
	if err != nil || strings.Contains(string(encoded), "Secret") || strings.Contains(string(encoded), "cHJpdmF0") || strings.Contains(string(encoded), "private-policy-fingerprint") {
		t.Fatal("key secret serialized")
	}
	assertHTTPOutcome(t, httpTestTransport().Invoke(context.Background(), httpTestInput()), entity.HookTransportError)
}

type httpTestErrorBody struct {
	err   error
	reads int
}

func (b *httpTestErrorBody) Read([]byte) (int, error) { b.reads++; return 0, b.err }
func (*httpTestErrorBody) Close() error               { return nil }

func TestHookHTTPReadDeadlineIsUncertainBeforeContextTimer(t *testing.T) {
	for _, tc := range []struct {
		status int
		err    error
		code   entity.HookOutcomeCode
	}{
		{200, os.ErrDeadlineExceeded, entity.HookTimeoutUncertain},
		{200, context.Canceled, entity.HookTimeoutUncertain},
		{200, io.ErrUnexpectedEOF, entity.HookTransportError},
		{503, os.ErrDeadlineExceeded, entity.HookHTTPError},
	} {
		body := &httpTestErrorBody{err: tc.err}
		_, result := readHTTPResponse(&http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: body})
		if result.Code != tc.code || result.HTTPStatus != tc.status {
			t.Fatalf("outcome=%+v want=%s", result, tc.code)
		}
		if body.reads != 1 {
			t.Fatal("failed body was reread")
		}
	}
}

func TestHookHTTPDNSFailureAndLiteralIP(t *testing.T) {
	for _, mode := range []string{"dns error", "empty dns", "forbidden literal", "public literal"} {
		t.Run(mode, func(t *testing.T) {
			tr, in := httpTestTransport(), httpTestInput()
			var lookups, dials atomic.Int32
			tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
				lookups.Add(1)
				if mode == "dns error" {
					return nil, errors.New("private-query resolver-private")
				}
				return nil, nil
			}
			tr.dial = func(_ context.Context, _, address string) (net.Conn, error) {
				dials.Add(1)
				if address != "203.0.113.10:443" {
					t.Error("literal IP not pinned")
				}
				return nil, errors.New("private-query")
			}
			want := entity.HookSecurityError
			if mode == "dns error" || mode == "public literal" {
				want = entity.HookTransportError
			}
			if strings.Contains(mode, "literal") {
				host := "127.0.0.1"
				if mode == "public literal" {
					host = "203.0.113.10"
				}
				in.Config.InvokeHTTPInfo.URL = gptr.Of("https://" + host + "/")
				tr.policies = func(context.Context) ([]EndpointPolicy, error) {
					p := httpTestPolicy()
					p.Host = host
					return []EndpointPolicy{p}, nil
				}
				tr.keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
					p := httpTestPolicy()
					p.Host = host
					return httpTestSigningKey(p, "test-key")
				})
			}
			assertHTTPOutcome(t, tr.Invoke(context.Background(), in), want)
			if strings.Contains(mode, "literal") && lookups.Load() != 0 {
				t.Fatal("literal host unexpectedly resolved")
			}
			if mode != "public literal" && dials.Load() != 0 {
				t.Fatal("invalid resolution dialed")
			}
			if mode == "public literal" && dials.Load() != 1 {
				t.Fatal("public literal not dialed")
			}
		})
	}
}

type httpTestFaultConn struct {
	*httpTestPeerConn
	deadlineErr error
}

func (c *httpTestFaultConn) SetDeadline(at time.Time) error {
	if c.deadlineErr != nil {
		return c.deadlineErr
	}
	return c.Conn.SetDeadline(at)
}

func TestHookHTTPDialFailureClosesAcquiredConnection(t *testing.T) {
	for _, mode := range []string{"nil conn", "conn and error", "nil peer", "deadline error", "late canceled conn"} {
		t.Run(mode, func(t *testing.T) {
			tr, in := httpTestTransport(), httpTestInput()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			closed := &atomic.Int32{}
			tr.dial = func(context.Context, string, string) (net.Conn, error) {
				if mode == "nil conn" {
					return nil, nil
				}
				client, server := net.Pipe()
				_ = server.Close()
				c := &httpTestFaultConn{httpTestPeerConn: &httpTestPeerConn{Conn: client, peer: &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 443}, closed: closed}}
				switch mode {
				case "conn and error":
					return c, errors.New("resolver-private")
				case "nil peer":
					c.peer = nil
				case "deadline error":
					c.deadlineErr = errors.New("private-query")
				case "late canceled conn":
					cancel()
				}
				return c, nil
			}
			want := entity.HookSecurityError
			if mode == "conn and error" || mode == "deadline error" {
				want = entity.HookTransportError
			}
			if mode == "late canceled conn" {
				want = entity.HookTimeoutUncertain
			}
			assertHTTPOutcome(t, tr.Invoke(ctx, in), want)
			if mode != "nil conn" {
				waitHTTPClosed(t, closed, 1)
			}
		})
	}
}

func TestHookHTTPDNSNativeTimeoutIsUncertain(t *testing.T) {
	tr := httpTestTransport()
	tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return nil, &net.DNSError{Err: "private-query", IsTimeout: true}
	}
	assertHTTPOutcome(t, tr.Invoke(context.Background(), httpTestInput()), entity.HookTimeoutUncertain)
}

func TestHookHTTPRetryAfterOverflowPreservesDomainDelay(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ name, raw, want string }{
		{"uint64 overflow", "18446744073709551616", "60"},
		{"trimmed overflow", " 18446744073709551616 ", "60"},
		{"128 digits", strings.Repeat("9", 128), "60"},
		{"129 bytes", strings.Repeat("9", 129), ""},
		{"invalid numeric prefix", "18446744073709551616x", ""},
		{"signed value", "+18446744073709551616", ""},
		{"negative value", "-1", ""},
		{"decimal", "1.5", ""},
		{"ordinary seconds", "30", "30"},
		{"date", "Wed, 21 Oct 2015 07:28:00 GMT", "Wed, 21 Oct 2015 07:28:00 GMT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := safeHTTPRetryAfter(tc.raw)
			if got != tc.want {
				t.Errorf("Retry-After=%q want=%q", got, tc.want)
			}
			if tc.want == "60" {
				for _, status := range []int{429, 503} {
					for attempt := int32(1); attempt <= 3; attempt++ {
						if direct := entity.HookRetryDelay(attempt, status, tc.raw, now); direct != time.Minute {
							t.Fatalf("domain contract changed: %s", direct)
						}
						if delay := entity.HookRetryDelay(attempt, status, got, now); delay != time.Minute {
							t.Errorf("attempt %d status %d delay=%s want=1m", attempt, status, delay)
						}
					}
				}
			}
		})
	}
}

type httpReviewPointerKeys struct {
	calls  *atomic.Int32
	policy EndpointPolicy
}

func (k *httpReviewPointerKeys) Resolve(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
	k.calls.Add(1)
	return httpTestSigningKey(k.policy, "test-key")
}

func TestHookHTTPTypedNilKeyResolverFailsClosed(t *testing.T) {
	for _, mode := range []string{"nil interface", "nil function", "nil pointer", "normal function", "normal pointer"} {
		t.Run(mode, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("missing dependency panicked: %v", recovered)
				}
			}()
			var keysCalled, lookups, dials, requests atomic.Int32
			fixture, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); writeHTTPSuccess(w) })
			policies, err := fixture.policies(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var keys hookcomponent.KeyResolver
			switch mode {
			case "nil function":
				keys = httpTestKeys(nil)
			case "nil pointer":
				keys = (*httpReviewPointerKeys)(nil)
			case "normal function":
				keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
					keysCalled.Add(1)
					return httpTestSigningKey(policies[0], "test-key")
				})
			case "normal pointer":
				keys = &httpReviewPointerKeys{calls: &keysCalled, policy: policies[0]}
			}
			tr := NewHTTPTransport(keys, fixture.policies)
			tr.rootCAs = fixture.rootCAs
			tr.lookup = func(ctx context.Context, network, host string) ([]netip.Addr, error) {
				lookups.Add(1)
				return fixture.lookup(ctx, network, host)
			}
			tr.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
				dials.Add(1)
				return fixture.dial(ctx, network, address)
			}
			if keysCalled.Load() != 0 || lookups.Load() != 0 || dials.Load() != 0 {
				t.Fatal("constructor eagerly invoked dependency")
			}
			want, calls := entity.HookSecurityError, int32(0)
			if strings.HasPrefix(mode, "normal") {
				want, calls = entity.HookSucceeded, 1
			}
			assertHTTPOutcome(t, tr.Invoke(context.Background(), in), want)
			if keysCalled.Load() != calls || lookups.Load() != calls || dials.Load() != calls || requests.Load() != calls {
				t.Fatalf("dependency/network counts: key=%d lookup=%d dial=%d HTTP=%d want=%d", keysCalled.Load(), lookups.Load(), dials.Load(), requests.Load(), calls)
			}
		})
	}
}
