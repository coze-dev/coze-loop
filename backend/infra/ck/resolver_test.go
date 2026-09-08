// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
	"gopkg.in/yaml.v3"
)

func TestNewCKFromConfig_ResolverReachesHTTP(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("NO_PROXY", "*")
	t.Setenv("no_proxy", "*")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "local ClickHouse HTTP fixture", http.StatusForbidden)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	var lookups atomic.Int32
	resolver := loopbackResolver(t, func(q dnsmessage.Question) bool {
		if q.Type == dnsmessage.TypeA {
			lookups.Add(1)
			return true
		}
		return false
	})
	cfg := &Config{
		Host:        net.JoinHostPort("clickhouse.invalid.", port),
		Protocol:    ProtocolHTTP,
		DialTimeout: time.Second,
		ReadTimeout: time.Second,
		Resolver:    resolver,
	}
	_, err = NewCKFromConfig(cfg)
	require.ErrorContains(t, err, "local ClickHouse HTTP fixture")
	require.Positive(t, requests.Load())
	require.Positive(t, lookups.Load())
}

func TestNewRetryDialer_ResolverRetries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		succeedOn int32
		wantDials int32
		wantError bool
	}{
		{name: "first success", succeedOn: 1, wantDials: 1},
		{name: "third success", succeedOn: 3, wantDials: 3},
		{name: "stop after three failures", succeedOn: 4, wantDials: 3, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			require.NoError(t, err)
			defer func() { _ = listener.Close() }()
			_, port, err := net.SplitHostPort(listener.Addr().String())
			require.NoError(t, err)
			var lookups atomic.Int32
			resolver := loopbackResolver(t, func(q dnsmessage.Question) bool {
				return q.Type == dnsmessage.TypeA && lookups.Add(1) >= tc.succeedOn
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			conn, err := newRetryDialer(time.Second, resolver)(ctx, net.JoinHostPort("clickhouse.invalid.", port))
			if tc.wantError {
				require.Nil(t, conn)
				var dnsErr *net.DNSError
				require.ErrorAs(t, err, &dnsErr)
				require.True(t, dnsErr.IsNotFound)
			} else {
				require.NoError(t, err)
				require.NoError(t, conn.Close())
			}
			require.Equal(t, tc.wantDials, lookups.Load())
		})
	}
}

func TestNewRetryDialer_NilResolver(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	before := net.DefaultResolver
	conn, err := newRetryDialer(time.Second, nil)(context.Background(), net.JoinHostPort("localhost", port))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.Same(t, before, net.DefaultResolver)
}

func TestNewRetryDialer_ResolverTimeoutAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		timeout       time.Duration
		parentTimeout time.Duration
		wantElapsed   time.Duration
	}{
		{name: "default five seconds per attempt", parentTimeout: 20 * time.Second, wantElapsed: 15 * time.Second},
		{name: "configured timeout per attempt", timeout: 40 * time.Millisecond, parentTimeout: time.Second, wantElapsed: 120 * time.Millisecond},
		{name: "parent deadline wins", timeout: time.Second, parentTimeout: 40 * time.Millisecond, wantElapsed: 40 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			}}
			ctx, cancel := context.WithTimeout(context.Background(), tc.parentTimeout)
			defer cancel()
			started := time.Now()
			conn, err := newRetryDialer(tc.timeout, resolver)(ctx, "clickhouse.invalid.:9000")
			elapsed := time.Since(started)
			require.Nil(t, conn)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.GreaterOrEqual(t, elapsed, tc.wantElapsed*9/10)
			require.Less(t, elapsed, tc.wantElapsed+2*time.Second)
		})
	}
	t.Run("parent cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resolver := &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, context.Canceled
		}}
		conn, err := newRetryDialer(0, resolver)(ctx, "clickhouse.invalid.:9000")
		require.Nil(t, conn)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestNewCKFromConfig_ResolverIsCodeOnly(t *testing.T) {
	cfg := Config{Resolver: &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, nil
	}}}
	for name, marshal := range map[string]func(any) ([]byte, error){"json": json.Marshal, "yaml": yaml.Marshal} {
		t.Run(name, func(t *testing.T) {
			data, err := marshal(cfg)
			require.NoError(t, err)
			require.NotContains(t, string(data), "Resolver")
			require.NotContains(t, string(data), "resolver")
		})
	}
}

func loopbackResolver(t *testing.T, answer func(dnsmessage.Question) bool) *net.Resolver {
	t.Helper()
	var workers sync.WaitGroup
	t.Cleanup(workers.Wait)
	return &net.Resolver{
		PreferGo: true,
		Dial: func(_ context.Context, _, _ string) (net.Conn, error) {
			client, server := net.Pipe()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { _ = server.Close() }()
				if err := server.SetDeadline(time.Now().Add(time.Second)); err != nil {
					t.Error(err)
					return
				}
				var size [2]byte
				if _, err := io.ReadFull(server, size[:]); err != nil {
					t.Error(err)
					return
				}
				wire := make([]byte, binary.BigEndian.Uint16(size[:]))
				if _, err := io.ReadFull(server, wire); err != nil {
					t.Error(err)
					return
				}
				var query dnsmessage.Message
				if err := query.Unpack(wire); err != nil {
					t.Error(err)
					return
				}
				response := dnsmessage.Message{
					Header:    dnsmessage.Header{ID: query.ID, Response: true, RecursionAvailable: true},
					Questions: query.Questions,
				}
				for _, q := range query.Questions {
					if answer(q) {
						response.Answers = append(response.Answers, dnsmessage.Resource{
							Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 1},
							Body:   &dnsmessage.AResource{A: [4]byte{127, 0, 0, 1}},
						})
					} else {
						response.RCode = dnsmessage.RCodeNameError
					}
				}
				wire, err := response.Pack()
				if err != nil {
					t.Error(err)
					return
				}
				binary.BigEndian.PutUint16(size[:], uint16(len(wire)))
				if _, err := server.Write(append(size[:], wire...)); err != nil {
					t.Error(err)
				}
			}()
			return client, nil
		},
	}
}
