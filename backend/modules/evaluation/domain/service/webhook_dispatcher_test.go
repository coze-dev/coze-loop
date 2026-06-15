// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"

	"go.uber.org/mock/gomock"
)

func TestMatchNotificationFilter_BitsUT(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil filter returns true", func(t *testing.T) {
		t.Parallel()
		assert.True(t, matchNotificationFilter(ctx, nil, entity.ExptStatus_Success))
	})

	t.Run("empty conditions returns true", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{FilterConditions: []*entity.NotificationFilterCondition{}}
		assert.True(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success))
	})

	t.Run("In operator matching status returns true", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_In,
					Value:    `["11","12"]`,
				},
			},
		}
		assert.True(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success)) // 11
	})

	t.Run("In operator non-matching status returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_In,
					Value:    `["11"]`,
				},
			},
		}
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Failed)) // 12
	})

	t.Run("NotIn operator matching status returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_NotIn,
					Value:    `["11"]`,
				},
			},
		}
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success))
	})

	t.Run("NotIn operator non-matching status returns true", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
					Operator: entity.NotificationOperatorType_NotIn,
					Value:    `["11"]`,
				},
			},
		}
		assert.True(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Failed))
	})

	t.Run("condition with non-ExptStatus field type is skipped, returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{
					Field:    &entity.NotificationFilterField{FieldType: 999}, // unknown field type
					Operator: entity.NotificationOperatorType_In,
					Value:    `["11"]`,
				},
			},
		}
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success))
	})

	t.Run("condition with nil field is skipped, returns false", func(t *testing.T) {
		t.Parallel()
		f := &entity.NotificationFilter{
			FilterConditions: []*entity.NotificationFilterCondition{
				{Field: nil, Operator: entity.NotificationOperatorType_In, Value: `["11"]`},
			},
		}
		assert.False(t, matchNotificationFilter(ctx, f, entity.ExptStatus_Success))
	})
}

func TestBuildWebhookPayload_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("Processing status maps to experiment.started", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 1, ExptRunID: gptr.Of(int64(10)), FromStatus: entity.ExptStatus_Pending, ToStatus: entity.ExptStatus_Processing, IdempotentKey: "expt_1_10_2_3"}
		expt := &entity.Experiment{ID: 1, Name: "test-expt"}
		payload := buildWebhookPayload(event, expt)
		assert.Equal(t, "experiment.started", payload["event_type"])
		assert.Equal(t, "expt_1_10_2_3", payload["delivery_id"])
	})

	t.Run("Success status maps to experiment.succeeded", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 2, ExptRunID: gptr.Of(int64(20)), FromStatus: entity.ExptStatus_Processing, ToStatus: entity.ExptStatus_Success}
		expt := &entity.Experiment{ID: 2, Name: "expt-2", Stats: &entity.ExptStats{SuccessItemCnt: 10, FailItemCnt: 2}}
		payload := buildWebhookPayload(event, expt)
		assert.Equal(t, "experiment.succeeded", payload["event_type"])
		data := payload["data"].(map[string]interface{})
		assert.Equal(t, "2", data["experiment_id"])
		assert.Equal(t, "expt-2", data["experiment_name"])
		progress := data["progress"].(map[string]interface{})
		assert.Equal(t, int32(12), progress["total"])
		assert.Equal(t, int32(10), progress["succeeded"])
		assert.Equal(t, int32(2), progress["failed"])
	})

	t.Run("nil ExptRunID uses 0", func(t *testing.T) {
		t.Parallel()
		event := &entity.ExptLifecycleEvent{ExptID: 3, FromStatus: entity.ExptStatus_Processing, ToStatus: entity.ExptStatus_Failed, IdempotentKey: "expt_3_0_3_12"}
		expt := &entity.Experiment{ID: 3, Name: "expt-3"}
		payload := buildWebhookPayload(event, expt)
		assert.Equal(t, "expt_3_0_3_12", payload["delivery_id"])
	})
}

func TestComputeHMACSHA256_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("deterministic output", func(t *testing.T) {
		t.Parallel()
		result1 := computeHMACSHA256("secret", "message")
		result2 := computeHMACSHA256("secret", "message")
		assert.Equal(t, result1, result2)
		assert.Len(t, result1, 64) // hex-encoded SHA256 = 64 chars
	})

	t.Run("empty secret produces valid hash", func(t *testing.T) {
		t.Parallel()
		result := computeHMACSHA256("", "test\nnonce\n")
		assert.Len(t, result, 64)
	})

	t.Run("different secrets produce different hashes", func(t *testing.T) {
		t.Parallel()
		r1 := computeHMACSHA256("secret1", "msg")
		r2 := computeHMACSHA256("secret2", "msg")
		assert.NotEqual(t, r1, r2)
	})
}

func TestParseWebhookURLs_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, parseWebhookURLs(nil))
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, parseWebhookURLs(gptr.Of("")))
	})

	t.Run("single URL", func(t *testing.T) {
		t.Parallel()
		urls := parseWebhookURLs(gptr.Of("https://example.com/hook"))
		assert.Equal(t, []string{"https://example.com/hook"}, urls)
	})

	t.Run("comma separated with spaces", func(t *testing.T) {
		t.Parallel()
		urls := parseWebhookURLs(gptr.Of("https://a.com , https://b.com, https://c.com "))
		assert.Equal(t, []string{"https://a.com", "https://b.com", "https://c.com"}, urls)
	})
}

func TestWebhookDispatcher_Dispatch_BitsUT(t *testing.T) {
	t.Parallel()

	t.Run("nil expt returns nil", func(t *testing.T) {
		t.Parallel()
		d := &WebhookDispatcher{}
		err := d.Dispatch(context.Background(), &entity.ExptLifecycleEvent{}, nil)
		assert.NoError(t, err)
	})

	t.Run("no NotificationConf returns nil", func(t *testing.T) {
		t.Parallel()
		d := &WebhookDispatcher{}
		err := d.Dispatch(context.Background(), &entity.ExptLifecycleEvent{}, &entity.Experiment{})
		assert.NoError(t, err)
	})

	t.Run("webhook disabled returns nil", func(t *testing.T) {
		t.Parallel()
		d := &WebhookDispatcher{}
		expt := &entity.Experiment{
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{Enable: false},
			},
		}
		err := d.Dispatch(context.Background(), &entity.ExptLifecycleEvent{}, expt)
		assert.NoError(t, err)
	})

	t.Run("filter no match returns nil", func(t *testing.T) {
		t.Parallel()
		d := &WebhookDispatcher{}
		expt := &entity.Experiment{
			NotificationConf: &entity.ExptNotificationConf{
				Filter: &entity.NotificationFilter{
					FilterConditions: []*entity.NotificationFilterCondition{
						{
							Field:    &entity.NotificationFilterField{FieldType: entity.NotificationFieldType_ExptStatus},
							Operator: entity.NotificationOperatorType_In,
							Value:    `["11"]`, // only Success
						},
					},
				},
				Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of("http://x.com")},
			},
		}
		event := &entity.ExptLifecycleEvent{ToStatus: entity.ExptStatus_Failed} // 12, not in filter
		err := d.Dispatch(context.Background(), event, expt)
		assert.NoError(t, err)
	})

	t.Run("successful dispatch sends POST with correct headers", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var capturedHeaders http.Header
		var capturedBody []byte
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedHeaders = r.Header.Clone()
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			capturedBody = buf[:n]
			w.WriteHeader(200)
		}))
		defer server.Close()

		mockPub := mocks.NewMockExptEventPublisher(ctrl)
		secretProvider := NewNoopWebhookSecretProvider()

		d := NewWebhookDispatcher(mockPub, secretProvider, nil)

		expt := &entity.Experiment{
			ID: 42, SpaceID: 100, Name: "webhook-test",
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
			},
		}
		event := &entity.ExptLifecycleEvent{
			ExptID: 42, SpaceID: 100, ExptRunID: gptr.Of(int64(5)),
			FromStatus: entity.ExptStatus_Processing, ToStatus: entity.ExptStatus_Success,
			IdempotentKey: "expt_42_5_3_11",
		}

		err := d.Dispatch(context.Background(), event, expt)
		assert.NoError(t, err)

		// Verify headers
		assert.NotEmpty(t, capturedHeaders.Get("X-CozeLoop-Timestamp"))
		assert.NotEmpty(t, capturedHeaders.Get("X-CozeLoop-Nonce"))
		assert.NotEmpty(t, capturedHeaders.Get("X-CozeLoop-Signature"))
		assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"))

		// Verify signature
		ts := capturedHeaders.Get("X-CozeLoop-Timestamp")
		nonce := capturedHeaders.Get("X-CozeLoop-Nonce")
		expectedSig := computeHMACSHA256("", ts+"\n"+nonce+"\n")
		assert.Equal(t, expectedSig, capturedHeaders.Get("X-CozeLoop-Signature"))

		// Verify body
		var body map[string]interface{}
		assert.NoError(t, json.Unmarshal(capturedBody, &body))
		assert.Equal(t, "experiment.succeeded", body["event_type"])
		assert.Equal(t, "experiment", body["resource_type"])
	})

	t.Run("failed POST triggers retry event", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer server.Close()

		mockPub := mocks.NewMockExptEventPublisher(ctrl)
		mockPub.EXPECT().PublishExptWebhookNotifyEvent(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, event *entity.WebhookRetryEvent, delay *time.Duration) error {
				assert.Equal(t, int64(42), event.ExptID)
				assert.Equal(t, int64(100), event.SpaceID)
				assert.Equal(t, 1, event.AttemptNum)
				assert.Equal(t, server.URL, event.WebhookURL)
				assert.Equal(t, time.Minute, *delay)
				return nil
			})

		d := NewWebhookDispatcher(mockPub, NewNoopWebhookSecretProvider(), nil)

		expt := &entity.Experiment{
			ID: 42, SpaceID: 100, Name: "fail-test",
			NotificationConf: &entity.ExptNotificationConf{
				Webhook: &entity.WebhookNotificationConf{Enable: true, Urls: gptr.Of(server.URL)},
			},
		}
		event := &entity.ExptLifecycleEvent{
			ExptID: 42, SpaceID: 100, ExptRunID: gptr.Of(int64(1)),
			FromStatus: entity.ExptStatus_Processing, ToStatus: entity.ExptStatus_Success,
			IdempotentKey: "expt_42_1_3_11",
		}

		err := d.Dispatch(context.Background(), event, expt)
		assert.NoError(t, err)
	})
}

func TestMapExptStatusToEventType_BitsUT(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status   entity.ExptStatus
		expected string
	}{
		{entity.ExptStatus_Processing, "experiment.started"},
		{entity.ExptStatus_Success, "experiment.succeeded"},
		{entity.ExptStatus_Failed, "experiment.failed"},
		{entity.ExptStatus_Terminated, "experiment.terminated"},
		{entity.ExptStatus_SystemTerminated, "experiment.terminated"},
		{entity.ExptStatus(999), "experiment.unknown"},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(int(c.status)), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expected, mapExptStatusToEventType(c.status))
		})
	}
}
