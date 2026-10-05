// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func signatureURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func signatureFixture(t *testing.T) SignatureInput {
	t.Helper()
	return SignatureInput{
		Method: "POST", URL: signatureURL(t, "https://hook.example/a%2fb/%E4%B8%AD?b=2&a=1&a=0"),
		UnixSeconds: 1750000000, Nonce: "nonce-001", OperationID: "hook_123", RawBody: []byte(`{"x":1}`),
	}
}

// Constants were independently calculated with Python hashlib/hmac, not the signer.
func TestSignV1Golden(t *testing.T) {
	in := signatureFixture(t)
	key := []byte("golden-test-key-only")
	beforeURL, beforeBody, beforeKey := *in.URL, bytes.Clone(in.RawBody), bytes.Clone(key)
	headers, err := SignV1(in, "key-2026", key)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"X-Fornax-Hook-Signature-Version": "v1",
		"X-Fornax-Hook-Key-Id":            "key-2026",
		"X-Fornax-Hook-Timestamp":         "1750000000",
		"X-Fornax-Hook-Nonce":             "nonce-001",
		"X-Fornax-Hook-Signature":         "edf6d46f59af9f358280d62a209b8b53fd81a9d472168df647caea11ae595e2c",
	}
	if len(headers) != len(want) {
		t.Fatalf("want exactly five headers, got %d", len(headers))
	}
	for name, value := range want {
		if got := headers.Values(name); !reflect.DeepEqual(got, []string{value}) {
			t.Errorf("%s = %v; want %q", name, got, value)
		}
	}
	wantMessage := "POST\n/a%2fb/%E4%B8%AD?b=2&a=1&a=0\n1750000000\nnonce-001\nhook_123\n5041bf1f713df204784353e82f6a4a535931cb64f1f4b4a5aeaffcb720918b22"
	message, err := signatureMessage(in)
	if err != nil || message != wantMessage {
		t.Fatalf("golden bytes = %q, err=%v", message, err)
	}
	if !reflect.DeepEqual(*in.URL, beforeURL) || !bytes.Equal(in.RawBody, beforeBody) || !bytes.Equal(key, beforeKey) {
		t.Fatal("signing mutated input")
	}
	for _, values := range headers {
		for _, value := range values {
			if strings.Contains(value, string(key)) {
				t.Fatal("secret leaked to headers")
			}
		}
	}
}

func TestSignV1PreservesRequestTarget(t *testing.T) {
	for _, tt := range []struct{ raw, want string }{
		{"https://hook.example", "f51a0bc5cfdca0d796cd51ae63d64325e48dea7d08a62be9f601de8586c4fb26"},
		{"https://hook.example?", "c5d45405d6431563c0976f794c41ac39c33ba15c73370f4ac11ac225f77a88c9"},
		{"https://hook.example/中", "5e16b09e8504f081eeac87ed4c9d1667325bfd53e1e0b498930c8bd05f1c209b"},
		{"https://hook.example/%e4%b8%ad/%2f", "d65c749c214eb1b314ee8a62a83e289550e9f722ab0fd8b6fb75dc1cebd47bac"},
		{"https://hook.example/?a=1&b=2", "55f5c1c242a79bfd71c94f39482e72a2a94e0542c0c593d94f8f214ee8c1ec18"},
		{"https://hook.example/?b=2&a=1", "94c93b3c998b36116a82820655ab48c9da6b7615f1529e9625f126c3112000d5"},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			in := signatureFixture(t)
			in.URL = signatureURL(t, tt.raw)
			headers, err := SignV1(in, "key-2026", []byte("golden-test-key-only"))
			if err != nil || headers.Get("X-Fornax-Hook-Signature") != tt.want {
				t.Fatalf("signature = %s, err=%v", headers.Get("X-Fornax-Hook-Signature"), err)
			}
		})
	}
}

func TestSignV1BindsAttemptAndBody(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*SignatureInput)
	}{
		{"body", func(in *SignatureInput) { in.RawBody = []byte(`{"x":2}`) }},
		{"body whitespace", func(in *SignatureInput) { in.RawBody = []byte(`{ "x":1}`) }},
		{"nonce", func(in *SignatureInput) { in.Nonce = "nonce-002" }},
		{"timestamp", func(in *SignatureInput) { in.UnixSeconds++ }},
		{"operation", func(in *SignatureInput) { in.OperationID = "hook_124" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := signatureFixture(t)
			tt.change(&in)
			headers, err := SignV1(in, "key-2026", []byte("golden-test-key-only"))
			if err != nil || len(headers.Get("X-Fornax-Hook-Signature")) != 64 || headers.Get("X-Fornax-Hook-Signature") == "edf6d46f59af9f358280d62a209b8b53fd81a9d472168df647caea11ae595e2c" {
				t.Fatalf("signature did not bind %s: %v", tt.name, err)
			}
		})
	}
}

func TestSignV1RejectsUnsafeInputsWithoutLeaking(t *testing.T) {
	const secret = "NEVER-EXPOSE-signing-secret"
	for _, tt := range []struct {
		name   string
		change func(*SignatureInput, *string, *[]byte)
	}{
		{"nil key", func(_ *SignatureInput, _ *string, key *[]byte) { *key = nil }},
		{"empty key id", func(_ *SignatureInput, id *string, _ *[]byte) { *id = "" }},
		{"key id injection", func(_ *SignatureInput, id *string, _ *[]byte) { *id = secret + "\r\nCookie: x" }},
		{"empty nonce", func(in *SignatureInput, _ *string, _ *[]byte) { in.Nonce = "" }},
		{"nonce injection", func(in *SignatureInput, _ *string, _ *[]byte) { in.Nonce = secret + "\nX: y" }},
		{"nonce whitespace", func(in *SignatureInput, _ *string, _ *[]byte) { in.Nonce = "  " }},
		{"nonce tab", func(in *SignatureInput, _ *string, _ *[]byte) { in.Nonce = "a\tb" }},
		{"nonce DEL", func(in *SignatureInput, _ *string, _ *[]byte) { in.Nonce = "a\x7fb" }},
		{"empty operation", func(in *SignatureInput, _ *string, _ *[]byte) { in.OperationID = "" }},
		{"operation injection", func(in *SignatureInput, _ *string, _ *[]byte) { in.OperationID = secret + "\n" }},
		{"operation limit", func(in *SignatureInput, _ *string, _ *[]byte) { in.OperationID = strings.Repeat("x", 129) }},
		{"invalid UTF8", func(in *SignatureInput, _ *string, _ *[]byte) { in.OperationID = "\xff" }},
		{"zero timestamp", func(in *SignatureInput, _ *string, _ *[]byte) { in.UnixSeconds = 0 }},
		{"negative timestamp", func(in *SignatureInput, _ *string, _ *[]byte) { in.UnixSeconds = -1 }},
		{"wrong method", func(in *SignatureInput, _ *string, _ *[]byte) { in.Method = "GET" }},
		{"lowercase method", func(in *SignatureInput, _ *string, _ *[]byte) { in.Method = "post" }},
		{"method injection", func(in *SignatureInput, _ *string, _ *[]byte) { in.Method = "POST\n" }},
		{"nil URL", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL = nil }},
		{"http", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.Scheme = "http" }},
		{"userinfo", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.User = url.UserPassword("user", secret) }},
		{"fragment", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.Fragment = secret }},
		{"opaque", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.Opaque = secret }},
		{"raw path mismatch", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.RawPath = "/different" }},
		{"bad raw escape", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.RawPath = "/%zz" }},
		{"bad query escape", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.RawQuery = "x=%zz" }},
		{"query injection", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.RawQuery = secret + "\r\nX: y" }},
		{"query space", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.RawQuery = "x=a b" }},
		{"query fragment", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.RawQuery = "x=#fragment" }},
		{"path control", func(in *SignatureInput, _ *string, _ *[]byte) { in.URL.Path = "/\n"; in.URL.RawPath = "" }},
		{"body limit", func(in *SignatureInput, _ *string, _ *[]byte) { in.RawBody = bytes.Repeat([]byte("x"), 64*1024+1) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in, id, key := signatureFixture(t), "test-key", []byte(secret)
			tt.change(&in, &id, &key)
			headers, err := SignV1(in, id, key)
			if err == nil || len(headers) != 0 {
				t.Fatal("unsafe input produced signature")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatal("error leaked sensitive input")
			}
		})
	}
}

func TestSignV1BodyBoundaryAndIndependentHeaders(t *testing.T) {
	for _, size := range []int{0, 64 * 1024} {
		in := signatureFixture(t)
		in.RawBody = make([]byte, size)
		in.OperationID = strings.Repeat("x", 128)
		first, err := SignV1(in, "test-key", []byte{0, 255, 1})
		if err != nil {
			t.Fatal(err)
		}
		first.Set("X-Fornax-Hook-Nonce", "changed")
		second, err := SignV1(in, "test-key", []byte{0, 255, 1})
		if err != nil || second.Get("X-Fornax-Hook-Nonce") != "nonce-001" {
			t.Fatal("signatures share mutable headers")
		}
	}
}

func TestSignV1KeyRotationKeepsOperationInput(t *testing.T) {
	in := signatureFixture(t)
	first, err := SignV1(in, "key-old", []byte("old-test-key"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := SignV1(in, "key-new", []byte("new-test-key"))
	if err != nil || len(second.Get("X-Fornax-Hook-Signature")) != 64 || first.Get("X-Fornax-Hook-Signature") == second.Get("X-Fornax-Hook-Signature") {
		t.Fatal("key rotation did not change signature")
	}
	if second.Get("X-Fornax-Hook-Key-Id") != "key-new" || in.OperationID != "hook_123" {
		t.Fatal("key selection or operation changed")
	}
}

func TestSignV1RejectsAmbiguousMixedUnicodeRawPath(t *testing.T) {
	in := signatureFixture(t)
	in.URL = signatureURL(t, "https://hook.example/中/%2f")
	if _, err := SignV1(in, "test-key", []byte("test-key-material")); err == nil {
		t.Fatal("Go would discard mixed RawPath and rewrite the escaped slash")
	}
}
