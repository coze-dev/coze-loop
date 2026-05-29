// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeSignature(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		timestamp string
		nonce     string
		want      string
	}{
		{
			name:      "known input produces expected HMAC-SHA256 signature",
			secret:    "my-secret-key",
			timestamp: "1700000000",
			nonce:     "abc123def456ghij",
			want: func() string {
				message := "1700000000" + "\n" + "abc123def456ghij" + "\n"
				mac := hmac.New(sha256.New, []byte("my-secret-key"))
				mac.Write([]byte(message))
				return hex.EncodeToString(mac.Sum(nil))
			}(),
		},
		{
			name:      "empty secret produces valid signature",
			secret:    "",
			timestamp: "1700000000",
			nonce:     "nonce123456789ab",
			want: func() string {
				message := "1700000000" + "\n" + "nonce123456789ab" + "\n"
				mac := hmac.New(sha256.New, []byte(""))
				mac.Write([]byte(message))
				return hex.EncodeToString(mac.Sum(nil))
			}(),
		},
		{
			name:      "different secrets produce different signatures",
			secret:    "secret-a",
			timestamp: "1700000000",
			nonce:     "nonce123456789ab",
			want: func() string {
				message := "1700000000" + "\n" + "nonce123456789ab" + "\n"
				mac := hmac.New(sha256.New, []byte("secret-a"))
				mac.Write([]byte(message))
				return hex.EncodeToString(mac.Sum(nil))
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeSignature(tt.secret, tt.timestamp, tt.nonce)
			assert.Equal(t, tt.want, got)
			// Signature should be 64 hex characters (SHA256 = 32 bytes = 64 hex chars)
			assert.Len(t, got, 64)
		})
	}

	// Additional test: different secrets yield different results
	t.Run("different secrets produce different results", func(t *testing.T) {
		sig1 := ComputeSignature("secret-a", "1700000000", "nonce123456789ab")
		sig2 := ComputeSignature("secret-b", "1700000000", "nonce123456789ab")
		assert.NotEqual(t, sig1, sig2)
	})
}

func TestGenerateNonce(t *testing.T) {
	t.Run("generates 16 character string", func(t *testing.T) {
		nonce := GenerateNonce()
		assert.Len(t, nonce, 16)
	})

	t.Run("only contains lowercase letters and digits", func(t *testing.T) {
		nonce := GenerateNonce()
		for _, c := range nonce {
			assert.True(t, (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'),
				"unexpected character: %c", c)
		}
	})

	t.Run("multiple calls produce different results (probabilistic)", func(t *testing.T) {
		nonces := make(map[string]bool)
		for i := 0; i < 100; i++ {
			nonces[GenerateNonce()] = true
		}
		// With 36^16 possible values, collisions are extremely unlikely
		assert.Greater(t, len(nonces), 90, "expected most nonces to be unique")
	})
}

func TestGenerateTimestamp(t *testing.T) {
	t.Run("returns valid unix timestamp string", func(t *testing.T) {
		before := time.Now().Unix()
		ts := GenerateTimestamp()
		after := time.Now().Unix()

		tsInt, err := strconv.ParseInt(ts, 10, 64)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, tsInt, before)
		assert.LessOrEqual(t, tsInt, after)
	})

	t.Run("timestamp string is non-empty", func(t *testing.T) {
		ts := GenerateTimestamp()
		assert.NotEmpty(t, ts)
	})
}

func TestSignRequest(t *testing.T) {
	t.Run("returns all three non-empty values", func(t *testing.T) {
		timestamp, nonce, signature := SignRequest("test-secret")
		assert.NotEmpty(t, timestamp)
		assert.NotEmpty(t, nonce)
		assert.NotEmpty(t, signature)
	})

	t.Run("timestamp is valid unix timestamp", func(t *testing.T) {
		timestamp, _, _ := SignRequest("test-secret")
		_, err := strconv.ParseInt(timestamp, 10, 64)
		assert.NoError(t, err)
	})

	t.Run("nonce is 16 characters", func(t *testing.T) {
		_, nonce, _ := SignRequest("test-secret")
		assert.Len(t, nonce, 16)
	})

	t.Run("signature is 64 hex characters", func(t *testing.T) {
		_, _, signature := SignRequest("test-secret")
		assert.Len(t, signature, 64)
	})

	t.Run("signature matches recomputed value", func(t *testing.T) {
		timestamp, nonce, signature := SignRequest("test-secret")
		expected := ComputeSignature("test-secret", timestamp, nonce)
		assert.Equal(t, expected, signature)
	})
}
