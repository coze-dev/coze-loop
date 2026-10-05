// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Only the test dialer maps an allowed synthetic IP to a real local TLS server.
// Production performs normal certificate verification and has no loopback bypass.
type httpTestPeerConn struct {
	net.Conn
	peer   net.Addr
	closed *atomic.Int32
}

func (c *httpTestPeerConn) RemoteAddr() net.Addr { return c.peer }
func (c *httpTestPeerConn) Close() error         { c.closed.Add(1); return c.Conn.Close() }

func waitHTTPClosed(t *testing.T, closed *atomic.Int32, count int32) {
	t.Helper()
	for deadline := time.Now().Add(time.Second); closed.Load() < count && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if closed.Load() < count {
		t.Fatal("connection did not close")
	}
}

func httpTLSFixture(t *testing.T, handler http.HandlerFunc) (*HTTPTransport, entity.HookTransportInput, *atomic.Int32) {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	srv.EnableHTTP2 = true
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	tr, in := httpTestTransport(), httpTestInput()
	in.Config.InvokeHTTPInfo.URL = gptr.Of("https://example.com:" + strconv.Itoa(port) + "/a%2fb/%E4%B8%AD?z=2&a=1&a=0")
	tr.rootCAs = x509.NewCertPool()
	tr.rootCAs.AddCert(srv.Certificate())
	tr.policies = func(context.Context) ([]EndpointPolicy, error) {
		p := httpTestPolicy()
		p.Port = uint16(port)
		return []EndpointPolicy{p}, nil
	}
	tr.keys = httpTestKeys(func(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error) {
		p := httpTestPolicy()
		p.Port = uint16(port)
		return httpTestSigningKey(p, "test-key")
	})
	closed := &atomic.Int32{}
	tr.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != netip.AddrPortFrom(netip.MustParseAddr("203.0.113.10"), uint16(port)).String() {
			return nil, errors.New("dial was not pinned to checked IP")
		}
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
		if err != nil {
			return nil, err
		}
		return &httpTestPeerConn{Conn: conn, peer: &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: port}, closed: closed}, nil
	}
	return tr, in, closed
}

func writeHTTPSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"succeeded","result":{"ok":"yes"}}`)
}

func TestHookHTTPTLSWireSignatureEnvironmentAndFreshness(t *testing.T) {
	for _, environment := range []entity.HookEnvironment{entity.HookEnvironmentProd, entity.HookEnvironmentPPE, entity.HookEnvironmentBOE} {
		t.Run(string(environment), func(t *testing.T) {
			type received struct {
				method, uri, body string
				headers           http.Header
				major             int
				sni               string
			}
			wire := make(chan received, 2)
			tr, in, closed := httpTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				wire <- received{r.Method, r.RequestURI, string(body), r.Header.Clone(), r.ProtoMajor, r.TLS.ServerName}
				writeHTTPSuccess(w)
			})
			in.Config.Environment = &environment
			lane := ""
			if environment != entity.HookEnvironmentProd {
				lane = "lane_exact"
				in.Config.Lane = &lane
			}
			basePolicies := tr.policies
			tr.policies = func(ctx context.Context) ([]EndpointPolicy, error) {
				p, err := basePolicies(ctx)
				p[0].Environment, p[0].Lane = string(environment), lane
				return p, err
			}
			var bindings []entity.HookKeyBinding
			keyPolicies, err := tr.policies(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			tr.keys = httpTestKeys(func(_ context.Context, b entity.HookKeyBinding) (entity.HookSigningKey, error) {
				bindings = append(bindings, b)
				return httpTestSigningKey(keyPolicies[0], "tls-key")
			})
			t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
			var nonces []string
			for i := 0; i < 2; i++ {
				start := time.Now()
				result := tr.Invoke(context.Background(), in)
				assertHTTPOutcome(t, result, entity.HookSucceeded)
				if result.LocalCompletedAt.Before(start) || result.LocalCompletedAt.After(time.Now()) {
					t.Fatal("completion is not local time")
				}
				if result.Response.GetResult_()["ok"] != "yes" {
					t.Fatal("missing parsed response")
				}
				got := <-wire
				if got.method != "POST" || got.uri != "/a%2fb/%E4%B8%AD?z=2&a=1&a=0" || got.body != requestGolden || got.major != 1 || got.sni != "example.com" {
					t.Fatalf("unexpected wire shape: method=%s uri=%s HTTP=%d SNI=%s", got.method, got.uri, got.major, got.sni)
				}
				for _, key := range []string{"Cookie", "Authorization", "Proxy-Authorization", "Idempotency-Key", "X-Idempotency-Key", "Accept-Encoding"} {
					if got.headers.Get(key) != "" {
						t.Fatalf("unexpected header %s", key)
					}
				}
				if got.headers.Get("Content-Type") != "application/json; charset=utf-8" || got.headers.Get("X-Fornax-Hook-Signature-Version") != "v1" || got.headers.Get("X-Fornax-Hook-Key-Id") != "tls-key" {
					t.Fatal("missing fixed headers")
				}
				if got.headers.Get("X-Tt-Env") != lane {
					t.Fatal("wrong target lane")
				}
				ppe := ""
				if environment == entity.HookEnvironmentPPE {
					ppe = "1"
				}
				if got.headers.Get("X-Use-Ppe") != ppe {
					t.Fatal("wrong PPE header")
				}
				// Independent receiver-side HMAC over actual wire bytes, not SignV1.
				var payload struct {
					OperationID string `json:"operation_id"`
				}
				if json.Unmarshal([]byte(got.body), &payload) != nil {
					t.Fatal("invalid body")
				}
				digest := sha256.Sum256([]byte(got.body))
				message := strings.Join([]string{"POST", got.uri, got.headers.Get("X-Fornax-Hook-Timestamp"), got.headers.Get("X-Fornax-Hook-Nonce"), payload.OperationID, hex.EncodeToString(digest[:])}, "\n")
				mac := hmac.New(sha256.New, []byte("private-test-secret"))
				_, _ = mac.Write([]byte(message))
				if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(got.headers.Get("X-Fornax-Hook-Signature"))) {
					t.Fatal("wire signature invalid")
				}
				nonces = append(nonces, got.headers.Get("X-Fornax-Hook-Nonce"))
			}
			if len(nonces[0]) < 32 || nonces[0] == nonces[1] {
				t.Fatal("nonce reused across Invoke")
			}
			if len(bindings) != 2 || bindings[0].WorkspaceID != 1 || bindings[0].URL != *in.Config.InvokeHTTPInfo.URL || bindings[0].Environment != environment || bindings[0].Lane != lane {
				t.Fatal("key not bound to exact target")
			}
			waitHTTPClosed(t, closed, 2)
		})
	}
}

func TestHookHTTPRedirectAndPostAreNeverReplayed(t *testing.T) {
	for _, mode := range []string{"redirect", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			var requests atomic.Int32
			tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if mode == "redirect" {
					w.Header().Set("Location", "/second")
					w.WriteHeader(307)
					return
				}
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
			})
			want := entity.HookTransportError
			if mode == "redirect" {
				want = entity.HookHTTPRedirectError
			}
			assertHTTPOutcome(t, tr.Invoke(context.Background(), in), want)
			if requests.Load() != 1 {
				t.Fatalf("body delivered %d times", requests.Load())
			}
		})
	}
}

func TestHookHTTPDeadlineIncludesEntireResponseBody(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{}, 1)
	tr, in, closed := httpTLSFixture(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":`)
		w.(http.Flusher).Flush()
		started <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	in.Remaining = 100 * time.Millisecond
	start := time.Now()
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookTimeoutUncertain)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("body deadline elapsed %s", elapsed)
	}
	select {
	case <-started:
	default:
		t.Fatal("timeout did not reach full-body phase")
	}
	for deadline := time.Now().Add(time.Second); closed.Load() == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if closed.Load() == 0 {
		t.Fatal("body connection did not close")
	}
}

func TestHookHTTPResponseBoundariesAndHTTPPriority(t *testing.T) {
	compressed := func(data []byte) []byte {
		var b bytes.Buffer
		z := gzip.NewWriter(&b)
		_, _ = z.Write(data)
		_ = z.Close()
		return b.Bytes()
	}
	valid := []byte(`{"status":"succeeded"}`)
	exact := append(append([]byte(nil), valid...), bytes.Repeat([]byte(" "), 32768-len(valid))...)
	for _, tc := range []struct {
		name                  string
		status                int
		encoding, contentType string
		body                  []byte
		code                  entity.HookOutcomeCode
		retry                 bool
	}{
		{"raw boundary", 200, "", "application/json", exact, entity.HookSucceeded, false},
		{"raw oversize", 200, "", "application/json", append(append([]byte(nil), exact...), ' '), entity.HookResponseTooLarge, false},
		{"gzip valid", 200, "gzip", "application/json", compressed(valid), entity.HookSucceeded, false},
		{"gzip decoded oversize", 200, "gzip", "application/json", compressed(bytes.Repeat([]byte(" "), 32769)), entity.HookResponseTooLarge, false},
		{"gzip raw oversize", 200, "gzip", "application/json", append(compressed(valid), make([]byte, 32769)...), entity.HookResponseTooLarge, false},
		{"gzip broken", 200, "gzip", "application/json", []byte("private-response"), entity.HookProtocolError, false},
		{"unknown encoding", 200, "br", "application/json", valid, entity.HookProtocolError, false},
		{"bad content type", 200, "", "text/plain", valid, entity.HookProtocolError, false},
		{"invalid utf8", 200, "", "application/json", []byte("{\"status\":\"\xff\"}"), entity.HookProtocolError, false},
		{"invalid json", 200, "", "application/json", []byte("private-response"), entity.HookProtocolError, false},
		{"duplicate JSON", 200, "", "application/json", []byte(`{"status":"succeeded","status":"failed"}`), entity.HookProtocolError, false},
		{"business retry", 200, "", "application/json", []byte(`{"status":"failed","error":{"message":"private-response","retryable":true}}`), entity.HookFailed, true},
		{"202", 202, "br", "text/plain", valid, entity.HookResultNotFinal, false},
		{"401", 401, "", "application/json", valid, entity.HookHTTPError, false},
		{"409", 409, "", "application/json", valid, entity.HookHTTPError, false},
		{"408", 408, "", "application/json", valid, entity.HookHTTPError, true},
		{"429 huge", 429, "br", "text/plain", bytes.Repeat([]byte("x"), 100000), entity.HookHTTPError, true},
		{"503 broken body", 503, "gzip", "text/plain", []byte("private-response"), entity.HookHTTPError, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, in, closed := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				if tc.encoding != "" {
					w.Header().Set("Content-Encoding", tc.encoding)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			})
			result := tr.Invoke(context.Background(), in)
			assertHTTPOutcome(t, result, tc.code)
			if result.Outcome.Retryable != tc.retry || result.Outcome.HTTPStatus != tc.status {
				t.Fatal("HTTP classification changed")
			}
			waitHTTPClosed(t, closed, 1)
		})
	}
}

func TestHookHTTPPeerMismatchAndCertificateValidation(t *testing.T) {
	for _, mode := range []string{"IP", "port", "certificate"} {
		t.Run(mode, func(t *testing.T) {
			var requests atomic.Int32
			tr, in, closed := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); writeHTTPSuccess(w) })
			base := tr.dial
			if mode == "certificate" {
				tr.rootCAs = x509.NewCertPool()
			} else {
				tr.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
					c, err := base(ctx, network, address)
					if err != nil {
						return nil, err
					}
					peer := c.(*httpTestPeerConn)
					a := *peer.peer.(*net.TCPAddr)
					if mode == "IP" {
						a.IP = net.ParseIP("127.0.0.1")
					} else {
						a.Port++
					}
					peer.peer = &a
					return c, nil
				}
			}
			assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSecurityError)
			if requests.Load() != 0 || closed.Load() == 0 {
				t.Fatal("peer/certificate failure executed HTTP or leaked connection")
			}
		})
	}
}

func TestHookHTTPDynamicPolicyAndDNS(t *testing.T) {
	tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeHTTPSuccess(w) })
	var policies, lookups atomic.Int32
	basePolicy, baseLookup := tr.policies, tr.lookup
	tr.policies = func(ctx context.Context) ([]EndpointPolicy, error) {
		if policies.Add(1) == 3 {
			return nil, nil
		}
		return basePolicy(ctx)
	}
	tr.lookup = func(ctx context.Context, network, host string) ([]netip.Addr, error) {
		lookups.Add(1)
		return baseLookup(ctx, network, host)
	}
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSucceeded)
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSucceeded)
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSecurityError)
	if policies.Load() != 3 || lookups.Load() != 2 {
		t.Fatal("policy/DNS was reused across invocations")
	}
}

func TestHookHTTPRetryAfterIsSafeForCompletion(t *testing.T) {
	for _, tc := range []struct{ value, want string }{{"30", "30"}, {"not-a-date", ""}, {strings.Repeat("9", 129), ""}, {"999999999999999999999999999999", "60"}, {"Wed, 21 Oct 2015 07:28:00 GMT", "Wed, 21 Oct 2015 07:28:00 GMT"}} {
		t.Run(tc.value[:min(len(tc.value), 30)], func(t *testing.T) {
			tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", tc.value)
				w.WriteHeader(429)
			})
			got := tr.Invoke(context.Background(), in)
			assertHTTPOutcome(t, got, entity.HookHTTPError)
			if got.RetryAfter != tc.want || len(got.RetryAfter) > 128 {
				t.Fatal("invalid or oversized Retry-After escaped")
			}
		})
	}
}

func TestHookHTTPTLSRetryAfterOverflowPreservesDomainDelay(t *testing.T) {
	for _, status := range []int{429, 503} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "18446744073709551616")
				w.WriteHeader(status)
			})
			got := tr.Invoke(context.Background(), in)
			assertHTTPOutcome(t, got, entity.HookHTTPError)
			if got.RetryAfter != "60" || got.Outcome.HTTPStatus != status || !got.Outcome.Retryable {
				t.Errorf("HTTP result=%+v RetryAfter=%q", got.Outcome, got.RetryAfter)
			}
			for attempt := int32(1); attempt <= 3; attempt++ {
				if delay := entity.HookRetryDelay(attempt, got.Outcome.HTTPStatus, got.RetryAfter, time.Now()); delay != time.Minute {
					t.Errorf("attempt %d delay=%s want=1m", attempt, delay)
				}
			}
		})
	}
}

func TestHookHTTPTCPFallbackDoesNotReplayHTTP(t *testing.T) {
	var requests atomic.Int32
	tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); writeHTTPSuccess(w) })
	base := tr.dial
	var attempts atomic.Int32
	tr.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("203.0.113.11"), netip.MustParseAddr("::ffff:203.0.113.10")}, nil
	}
	tr.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		attempts.Add(1)
		if strings.HasPrefix(address, "203.0.113.11:") {
			return nil, errors.New("first TCP unavailable")
		}
		return base(ctx, network, address)
	}
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookSucceeded)
	if requests.Load() != 1 || attempts.Load() != 2 {
		t.Fatal("TCP fallback changed HTTP attempt count")
	}
}

func TestHookHTTPTLSHandshakeDeadline(t *testing.T) {
	tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, _ *http.Request) { writeHTTPSuccess(w) })
	in.Remaining = 80 * time.Millisecond
	closed := &atomic.Int32{}
	tr.dial = func(_ context.Context, _, address string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() { _, _ = io.Copy(io.Discard, server); _ = server.Close() }()
		peer, _ := net.ResolveTCPAddr("tcp", address)
		return &httpTestPeerConn{Conn: client, peer: peer, closed: closed}, nil
	}
	start := time.Now()
	assertHTTPOutcome(t, tr.Invoke(context.Background(), in), entity.HookTimeoutUncertain)
	if time.Since(start) > time.Second {
		t.Fatal("TLS handshake exceeded budget")
	}
	for deadline := time.Now().Add(time.Second); closed.Load() == 0 && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if closed.Load() == 0 {
		t.Fatal("timed-out TLS connection leaked")
	}
}

func TestHookHTTPPathAndLocalClockPreserved(t *testing.T) {
	for _, path := range []string{"", "?", "/%e4%b8%ad/%2f?b=2&a=1", "/中?"} {
		t.Run(path, func(t *testing.T) {
			uris := make(chan string, 1)
			tr, in, _ := httpTLSFixture(t, func(w http.ResponseWriter, r *http.Request) { uris <- r.RequestURI; writeHTTPSuccess(w) })
			u := signatureURL(t, *in.Config.InvokeHTTPInfo.URL)
			in.Config.InvokeHTTPInfo.URL = gptr.Of("https://" + u.Host + path)
			local := time.Now()
			tr.now = func() time.Time { return local }
			tr.nonce = func() (string, error) { return "fixed-test-nonce", nil }
			got := tr.Invoke(context.Background(), in)
			assertHTTPOutcome(t, got, entity.HookSucceeded)
			if got.LocalCompletedAt != local {
				t.Fatal("local monotonic component changed")
			}
			want := path
			if path == "" {
				want = "/"
			}
			if path == "?" {
				want = "/?"
			}
			if path == "/中?" {
				want = "/%E4%B8%AD?"
			}
			if <-uris != want {
				t.Fatal("raw request target changed")
			}
		})
	}
}
