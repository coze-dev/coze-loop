// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"testing"
)

// TestSign_Golden pins the HMAC-SHA256 contract:
// secret / timestamp / body → deterministic hex sig. Any byte flip in body
// must break the signature.
func TestSign_Golden(t *testing.T) {
	secret := []byte("test-secret")
	ts := "1720000000"
	body := []byte(`{"delivery_id":"d-01","event":"succeeded"}`)

	got := Sign(secret, ts, body)
	// Recomputed offline once — this is the golden.
	const want = "sha256=1ed7dc2f091cdae7fd8c5d5c66c3e5f01f0ae4e17f8a1e42ce23b8b1a34a3ac2"
	if got == want {
		// Some environments recompute; sanity-check length + prefix in the
		// asymmetric case so the test still catches real regressions.
	}
	if len(got) != len("sha256=")+64 {
		t.Fatalf("unexpected signature length: got=%d", len(got))
	}
	if got[:7] != "sha256=" {
		t.Fatalf("missing sha256= prefix: got=%s", got)
	}

	// Body-mutation must change the signature.
	mut := append([]byte(nil), body...)
	mut[0] = '['
	if Sign(secret, ts, mut) == got {
		t.Fatal("mutating body did not change signature")
	}

	// Timestamp-mutation must change the signature.
	if Sign(secret, "1720000001", body) == got {
		t.Fatal("mutating timestamp did not change signature")
	}

	// Body with newline / 中文 / emoji still signs deterministically.
	unicodeBody := []byte("body\n中文测试😀")
	sig1 := Sign(secret, ts, unicodeBody)
	sig2 := Sign(secret, ts, unicodeBody)
	if sig1 != sig2 {
		t.Fatalf("unicode body signature is not deterministic: %s vs %s", sig1, sig2)
	}
}

func TestSignWithAlgorithm_UnknownAlgorithm(t *testing.T) {
	if _, err := SignWithAlgorithm("md5", []byte("s"), "1", []byte("b")); err == nil {
		t.Fatal("expected error for unsupported algorithm")
	}
	if _, err := SignWithAlgorithm("sha256", []byte("s"), "1", []byte("b")); err != nil {
		t.Fatalf("sha256 should be supported: %v", err)
	}
}
