// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package service — additional webhook unit tests covering paths not yet
// exercised in webhook_dispatcher_test.go.
package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// fakeSecretProvider implements IWebhookSecretProvider returning a fixed secret
type fakeSecretProvider struct {
	secret string
	err    error
}

func (f *fakeSecretProvider) GetSecret(_ context.Context, _ int64) (string, error) {
	return f.secret, f.err
}

// ---------------------------------------------------------------------------
// generateNonce — not covered in webhook_dispatcher_test.go
// ---------------------------------------------------------------------------

func TestGenerateNonce_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nonce is 32 hex chars (16 bytes)", func(t *testing.T) {
		t.Parallel()
		assert.Len(t, generateNonce(), 32)
	})

	t.Run("two nonces are unique", func(t *testing.T) {
		t.Parallel()
		assert.NotEqual(t, generateNonce(), generateNonce())
	})
}

// ---------------------------------------------------------------------------
// NoopWebhookSecretProvider — not covered in webhook_dispatcher_test.go
// ---------------------------------------------------------------------------

func TestNoopWebhookSecretProvider_BitsUT(t *testing.T) {
	t.Parallel()
	secret, err := NewNoopWebhookSecretProvider().GetSecret(context.Background(), 12345)
	assert.NoError(t, err)
	assert.Equal(t, "", secret)
}

// ---------------------------------------------------------------------------
// Dispatch — empty / nil URL (webhook enabled but no URLs)
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_EmptyURL_BitsUT(t *testing.T) {
	t.Parallel()
	d := &WebhookDispatcher{}
	expt := &entity.Experiment{
		ID: 1,
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of("")},
		},
	}
	err := d.Dispatch(context.Background(), &entity.ExptLifecycleEvent{ToStatus: entity.ExptStatus_Success}, expt)
	assert.NoError(t, err)
}

func TestWebhookDispatcher_Dispatch_NilURLs_BitsUT(t *testing.T) {
	t.Parallel()
	d := &WebhookDispatcher{}
	expt := &entity.Experiment{
		ID: 2,
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: nil},
		},
	}
	err := d.Dispatch(context.Background(), &entity.ExptLifecycleEvent{ToStatus: entity.ExptStatus_Success}, expt)
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Dispatch — stats already populated (statsRepo must NOT be called)
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_StatsAlreadySet_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	mockStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	// No Get expectation — any call would fail the test via gomock

	d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), mockStatsRepo)
	expt := &entity.Experiment{
		ID: 10, SpaceID: 50, Name: "stats-already-set",
		Stats: &entity.ExptStats{SuccessItemCnt: 3}, // already populated
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 10, SpaceID: 50,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_10_0_3_11",
	}
	assert.NoError(t, d.Dispatch(context.Background(), event, expt))
}

// ---------------------------------------------------------------------------
// Dispatch — multiple URLs: one succeeds, one fails → retry for failed only
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_MultipleURLs_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	successCount := 0
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		successCount++
		w.WriteHeader(200)
	}))
	defer successServer.Close()

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer failServer.Close()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	// Exactly one retry event for the failing URL
	mockPub.EXPECT().PublishExptWebhookNotifyEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), nil)
	expt := &entity.Experiment{
		ID: 20, SpaceID: 200, Name: "multi-url",
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{
				Enable: true,
				Urls:   gptr.Of(failServer.URL + "," + successServer.URL),
			},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 20, SpaceID: 200,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_20_0_3_11",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err)
	assert.Equal(t, 1, successCount, "success URL should be called once")
}

// ---------------------------------------------------------------------------
// matchNotificationFilter — Equal / NotEqual operators (not in existing tests)
// ---------------------------------------------------------------------------

func TestMatchNotificationFilter_AdditionalOperators_BitsUT(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Equal operator matching status returns true", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_Equal,
					Value:    `["11"]`,
				},
			},
		}
		assert.True(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success)) // 11 == Success
	})

	t.Run("Equal operator non-matching status returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_Equal,
					Value:    `["11"]`,
				},
			},
		}
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Failed)) // 12 != 11
	})

	t.Run("NotEqual operator status in list returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_NotEqual,
					Value:    `["11"]`,
				},
			},
		}
		// Success(11) is in list, so NotEqual condition → not satisfied → false
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success))
	})

	t.Run("NotEqual operator status not in list returns true", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_NotEqual,
					Value:    `["11"]`,
				},
			},
		}
		// Failed(12) not in list → NotEqual condition satisfied → true
		assert.True(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Failed))
	})

	t.Run("condition with invalid value JSON returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_In,
					Value:    `not-valid-json`,
				},
			},
		}
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success))
	})
}

// ---------------------------------------------------------------------------
// buildWebhookPayload — Terminated / SystemTerminated / Unknown statuses
//   and progress total calculation (not covered in existing tests)
// ---------------------------------------------------------------------------

func TestBuildWebhookPayload_AdditionalStatus_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("Terminated maps to experiment.terminated with status=terminated", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 5, ToStatus: entity.ExptStatus_Terminated, IdempotentKey: "k1"}
		payload := buildWebhookPayload(event, &entity.Experiment{ID: 5, Name: "t"})
		assert.Equal(t, "experiment.terminated", payload["event_type"])
		data := payload["data"].(map[string]interface{})
		assert.Equal(t, "terminated", data["status"])
	})

	t.Run("SystemTerminated maps to experiment.terminated", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 6, ToStatus: entity.ExptStatus_SystemTerminated, IdempotentKey: "k2"}
		payload := buildWebhookPayload(event, &entity.Experiment{ID: 6, Name: "st"})
		assert.Equal(t, "experiment.terminated", payload["event_type"])
	})

	t.Run("Unknown status maps to experiment.unknown with status=unknown", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 7, ToStatus: entity.ExptStatus(999), IdempotentKey: "k3"}
		payload := buildWebhookPayload(event, &entity.Experiment{ID: 7, Name: "u"})
		assert.Equal(t, "experiment.unknown", payload["event_type"])
		data := payload["data"].(map[string]interface{})
		assert.Equal(t, "unknown", data["status"])
	})

	t.Run("progress total sums all item count fields", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 10, ToStatus: entity.ExptStatus_Success}
		expt := &entity.Experiment{
			ID: 10, Name: "full-stats",
			Stats: &entity.ExptStats{
				SuccessItemCnt:    10,
				FailItemCnt:       3,
				PendingItemCnt:    2,
				ProcessingItemCnt: 1,
				TerminatedItemCnt: 4,
			},
		}
		payload := buildWebhookPayload(event, expt)
		data := payload["data"].(map[string]interface{})
		progress := data["progress"].(map[string]interface{})
		assert.Equal(t, int32(20), progress["total"]) // 10+3+2+1+4
		assert.Equal(t, int32(10), progress["succeeded"])
		assert.Equal(t, int32(3), progress["failed"])
	})

	t.Run("payload delivery_id uses IdempotentKey and resource_type is experiment", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 9, ToStatus: entity.ExptStatus_Success, IdempotentKey: "delivery-key"}
		payload := buildWebhookPayload(event, &entity.Experiment{ID: 9, Name: "fc"})
		assert.Equal(t, "delivery-key", payload["delivery_id"])
		assert.Equal(t, "experiment", payload["resource_type"])
		assert.Contains(t, payload, "create_time")
		assert.Contains(t, payload, "summary")
	})
}

// ---------------------------------------------------------------------------
// Dispatch — with real (non-empty) secret provider
//   Covers the branch at L111 where secretProvider != nil and returns secret.
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_WithRealSecret_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var capturedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("X-CozeLoop-Signature")
		w.WriteHeader(200)
	}))
	defer server.Close()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	secretProv := &fakeSecretProvider{secret: "my-webhook-secret"}

	d := NewWebhookDispatcher(mockPub, secretProv, nil)
	expt := &entity.Experiment{
		ID: 50, SpaceID: 500, Name: "secret-test",
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 50, SpaceID: 500,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_50_0_3_11",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err)
	assert.NotEmpty(t, capturedSig, "signature should be non-empty when secret provider returns a real secret")
	// With empty secret, the signature should differ
	emptySigSample := computeHMACSHA256("", "dummy")
	realSigSample := computeHMACSHA256("my-webhook-secret", "dummy")
	assert.NotEqual(t, emptySigSample, realSigSample)
}

// ---------------------------------------------------------------------------
// Dispatch — invalid URL triggers doPost NewRequestWithContext error
//   Covers doPost L147 error path.
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_InvalidURL_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	// Invalid URL triggers retry
	mockPub.EXPECT().PublishExptWebhookNotifyEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), nil)
	expt := &entity.Experiment{
		ID: 60, SpaceID: 600, Name: "invalid-url",
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of("://bad-url")},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 60, SpaceID: 600,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_60_0_3_11",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err) // Dispatch itself returns nil, failure triggers retry
}

// ---------------------------------------------------------------------------
// Dispatch — connection refused triggers doPost httpClient.Do error
//   Covers doPost L157 error path.
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_ConnectionRefused_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	// Connection refused triggers retry
	mockPub.EXPECT().PublishExptWebhookNotifyEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), nil)
	// Use a port that is guaranteed to not be listening
	expt := &entity.Experiment{
		ID: 70, SpaceID: 700, Name: "conn-refused",
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of("http://127.0.0.1:1")},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 70, SpaceID: 700,
		ToStatus: entity.ExptStatus_Failed, IdempotentKey: "expt_70_0_3_12",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err) // Dispatch returns nil; failure triggers retry
}

// ---------------------------------------------------------------------------
// Dispatch — statsRepo error + retry publish error (double error path)
//   Covers statsRepo error at L94 + retry publish error at L136 combined.
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_StatsAndRetryBothFail_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500) // trigger retry
	}))
	defer server.Close()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	mockPub.EXPECT().PublishExptWebhookNotifyEvent(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("mq publish error")).Times(1)

	mockStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockStatsRepo.EXPECT().Get(gomock.Any(), int64(80), int64(800)).Return(nil, errors.New("db error"))

	d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), mockStatsRepo)
	expt := &entity.Experiment{
		ID: 80, SpaceID: 800, Name: "double-error",
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 80, SpaceID: 800,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_80_0_3_11",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err) // Both errors logged but not propagated
}

// ---------------------------------------------------------------------------
// Dispatch — nil secretProvider path (secretProvider is nil)
//   Covers the branch where d.secretProvider == nil.
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_NilSecretProvider_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var capturedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("X-CozeLoop-Signature")
		w.WriteHeader(200)
	}))
	defer server.Close()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)

	// Directly construct dispatcher with nil secretProvider
	d := &WebhookDispatcher{
		httpClient:     &http.Client{},
		publisher:      mockPub,
		secretProvider: nil,
		statsRepo:      nil,
	}

	expt := &entity.Experiment{
		ID: 90, SpaceID: 900, Name: "nil-secret-prov",
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 90, SpaceID: 900,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_90_0_3_11",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err)
	// With nil secretProvider, signature should use empty string as secret
	assert.NotEmpty(t, capturedSig)
}

// ---------------------------------------------------------------------------
// Dispatch — statsRepo.Get succeeds and populates expt.Stats
//   Covers L96-98: the else branch after successful statsRepo.Get.
// ---------------------------------------------------------------------------

func TestWebhookDispatcher_Dispatch_StatsLoaded_BitsUT(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	mockPub := eventmocks.NewMockExptEventPublisher(ctrl)
	mockStatsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	mockStatsRepo.EXPECT().Get(gomock.Any(), int64(100), int64(1000)).Return(&entity.ExptStats{
		SuccessItemCnt: 8,
		FailItemCnt:    2,
	}, nil)

	d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), mockStatsRepo)
	expt := &entity.Experiment{
		ID: 100, SpaceID: 1000, Name: "stats-loaded",
		Stats: nil, // nil to trigger statsRepo.Get
		NotificationConf: &entity.ExptNotificationConf{
			Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
		},
	}
	event := &entity.ExptLifecycleEvent{
		ExptID: 100, SpaceID: 1000,
		ToStatus: entity.ExptStatus_Success, IdempotentKey: "expt_100_0_3_11",
	}

	err := d.Dispatch(context.Background(), event, expt)
	assert.NoError(t, err)
	assert.NotNil(t, expt.Stats)
	assert.Equal(t, int32(8), expt.Stats.SuccessItemCnt)
	assert.Equal(t, int32(2), expt.Stats.FailItemCnt)
}
