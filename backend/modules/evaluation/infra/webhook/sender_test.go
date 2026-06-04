// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestSenderSignsTimestampAndBody(t *testing.T) {
	secret := "space-signing-secret"
	payload := &entity.WebhookPayload{
		DeliveryID: "delivery-1",
		Event:      entity.WebhookEventSucceeded,
		Timestamp:  1710000000,
		Experiment: &entity.WebhookExptInfo{
			ID:     "123",
			Name:   "experiment",
			Status: "success",
		},
	}

	var handled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handled = true
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		timestamp := r.Header.Get(headerTimestamp)
		signature := r.Header.Get(headerSignature)
		require.NotEmpty(t, timestamp)
		require.NotEmpty(t, signature)
		assert.Equal(t, computeSignature(secret, timestamp, body), signature)
		assert.Equal(t, signature, r.Header.Get(headerFornaxSignature))
		assert.Equal(t, payload.DeliveryID, r.Header.Get(headerDeliveryID))
		assert.Equal(t, payload.DeliveryID, r.Header.Get(headerFornaxDelivery))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWebhookSenderWithConf(
		&entity.WebhookRetryConf{HTTPTimeout: time.Second},
		&entity.WebhookSecurityConf{},
	)
	result := sender.Send(context.Background(), server.URL, payload, secret)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.True(t, handled)
}
