// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSign_Golden pins the HMAC-SHA256 contract:
// secret / timestamp / body → deterministic hex sig. Any byte flip in body
// must break the signature.
func TestSign_Golden(t *testing.T) {
	secret := []byte("test-secret")
	ts := "1720000000"
	body := []byte(`{"delivery_id":"d-01","event":"succeeded"}`)

	got := Sign(secret, ts, body)
	// Recomputed via:
	//   printf '1720000000\n{"delivery_id":"d-01","event":"succeeded"}' \
	//     | openssl dgst -sha256 -hmac 'test-secret'
	const want = "sha256=7b1b2ebb7e6f9aaecf61c3e79182272cf79ab18d511796c51edd1330d63a767b"
	require.Equal(t, want, got, "HMAC golden mismatch — signer contract regressed")

	// Body-mutation must change the signature (NEG-01 unit protection).
	mut := append([]byte(nil), body...)
	mut[0] = '['
	mutSig := Sign(secret, ts, mut)
	require.NotEqual(t, got, mutSig, "mutating body did not change signature")

	// Timestamp-mutation must change the signature.
	tsSig := Sign(secret, "1720000001", body)
	require.NotEqual(t, got, tsSig, "mutating timestamp did not change signature")

	// Body with newline / 中文 / emoji still signs deterministically.
	unicodeBody := []byte("body\n中文测试😀")
	sig1 := Sign(secret, ts, unicodeBody)
	sig2 := Sign(secret, ts, unicodeBody)
	require.Equal(t, sig1, sig2, "unicode body signature is not deterministic")
}

func TestSignWithAlgorithm_UnknownAlgorithm(t *testing.T) {
	if _, err := SignWithAlgorithm("md5", []byte("s"), "1", []byte("b")); err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
	if _, err := SignWithAlgorithm("sha256", []byte("s"), "1", []byte("b")); err != nil {
		t.Fatalf("sha256 should be supported: %v", err)
	}
}
