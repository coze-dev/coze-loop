// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	pkgjson "github.com/coze-dev/coze-loop/backend/pkg/json"
)

// mockSecretProvider 模拟密钥提供者
type mockSecretProvider struct {
	secrets map[int64]string
}

func (m *mockSecretProvider) GetSecret(ctx context.Context, spaceID int64) (string, error) {
	return m.secrets[spaceID], nil
}

// mockPublisher 模拟事件发布者
type mockPublisher struct {
	publishedEvents []*entity.WebhookRetryEvent
}

func (m *mockPublisher) PublishExptScheduleEvent(ctx context.Context, event *entity.ExptScheduleEvent, duration *time.Duration) error {
	return nil
}

func (m *mockPublisher) PublishExptRecordEvalEvent(ctx context.Context, event *entity.ExptItemEvalEvent, duration *time.Duration, modifyFunc func(event *entity.ExptItemEvalEvent)) error {
	return nil
}

func (m *mockPublisher) BatchPublishExptRecordEvalEvent(ctx context.Context, events []*entity.ExptItemEvalEvent, duration *time.Duration) error {
	return nil
}

func (m *mockPublisher) PublishExptAggrCalculateEvent(ctx context.Context, events []*entity.AggrCalculateEvent, duration *time.Duration) error {
	return nil
}

func (m *mockPublisher) PublishExptOnlineEvalResult(ctx context.Context, events *entity.OnlineExptEvalResultEvent, duration *time.Duration) error {
	return nil
}

func (m *mockPublisher) PublishExptTurnResultFilterEvent(ctx context.Context, event *entity.ExptTurnResultFilterEvent, duration *time.Duration) error {
	return nil
}

func (m *mockPublisher) PublishExptExportCSVEvent(ctx context.Context, events *entity.ExportCSVEvent, duration *time.Duration) error {
	return nil
}

func (m *mockPublisher) PublishExptLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent, duration *time.Duration, idempotentKey string) error {
	return nil
}

func (m *mockPublisher) PublishExptWebhookNotifyEvent(ctx context.Context, event *entity.WebhookRetryEvent, duration *time.Duration) error {
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

// verifySignature 接收方验签逻辑
func verifySignature(secret, timestamp, nonce, gotSignature string) bool {
	signMessage := timestamp + "\n" + nonce + "\n"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signMessage))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(gotSignature), []byte(expectedSig))
}

// TestWebhookRetryConsumer_SignatureVerification 测试重试消费者发送的请求能通过接收方验签
func TestWebhookRetryConsumer_SignatureVerification(t *testing.T) {
	secret := "test-space-secret-key-123"
	var spaceID int64 = 10001

	// 模拟接收方：校验签名 + 返回 200
	var receivedHeaders http.Header
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		receivedBody, _ = io.ReadAll(r.Body)

		// 接收方验签
		ts := r.Header.Get("X-CozeLoop-Timestamp")
		nonce := r.Header.Get("X-CozeLoop-Nonce")
		sig := r.Header.Get("X-CozeLoop-Signature")

		if !verifySignature(secret, ts, nonce, sig) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("invalid signature"))
			return
		}

		// 验证时间戳防重放（5 分钟容忍）
		tsInt, _ := strconv.ParseInt(ts, 10, 64)
		if abs64(time.Now().Unix()-tsInt) > 300 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("request expired"))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 构造 WebhookRetryConsumer
	provider := &mockSecretProvider{secrets: map[int64]string{spaceID: secret}}
	publisher := &mockPublisher{}
	consumer := NewWebhookRetryConsumer(publisher, provider)

	// 构造消息
	payload := `{"event_type":"experiment.completed","experiment_id":"expt_123","delivery_id":"dlv_001"}`
	retryEvent := &entity.WebhookRetryEvent{
		ExptID:     123,
		SpaceID:    spaceID,
		DeliveryID: "dlv_001",
		WebhookURL: server.URL,
		Payload:    payload,
		AttemptNum: 1,
	}
	msgBody, err := pkgjson.Marshal(retryEvent)
	require.NoError(t, err)

	msg := &mq.MessageExt{
		Message: mq.Message{Body: msgBody},
		MsgID:   "msg_001",
	}

	// 执行
	err = consumer.HandleMessage(context.Background(), msg)

	// 验证
	assert.NoError(t, err)
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.NotEmpty(t, receivedHeaders.Get("X-CozeLoop-Timestamp"))
	assert.NotEmpty(t, receivedHeaders.Get("X-CozeLoop-Nonce"))
	assert.NotEmpty(t, receivedHeaders.Get("X-CozeLoop-Signature"))
	assert.JSONEq(t, payload, string(receivedBody))
}

// TestWebhookRetryConsumer_InvalidSignature 测试使用错误密钥时验签失败，触发重试
func TestWebhookRetryConsumer_InvalidSignature(t *testing.T) {
	realSecret := "real-secret-key"
	wrongSecret := "wrong-secret-key"
	var spaceID int64 = 10002

	// 接收方使用 realSecret 验签
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := r.Header.Get("X-CozeLoop-Timestamp")
		nonce := r.Header.Get("X-CozeLoop-Nonce")
		sig := r.Header.Get("X-CozeLoop-Signature")

		if !verifySignature(realSecret, ts, nonce, sig) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 消费者使用 wrongSecret 签名
	provider := &mockSecretProvider{secrets: map[int64]string{spaceID: wrongSecret}}
	publisher := &mockPublisher{}
	consumer := NewWebhookRetryConsumer(publisher, provider)

	retryEvent := &entity.WebhookRetryEvent{
		ExptID:     456,
		SpaceID:    spaceID,
		DeliveryID: "dlv_002",
		WebhookURL: server.URL,
		Payload:    `{"event_type":"experiment.failed"}`,
		AttemptNum: 1,
	}
	msgBody, _ := pkgjson.Marshal(retryEvent)
	msg := &mq.MessageExt{
		Message: mq.Message{Body: msgBody},
		MsgID:   "msg_002",
	}

	// 执行
	err := consumer.HandleMessage(context.Background(), msg)

	// 验证：请求失败（401），应发布下一次重试事件
	assert.NoError(t, err) // consumer 本身返回 nil（不让 MQ 自动重试）
	assert.Len(t, publisher.publishedEvents, 1)
	assert.Equal(t, 2, publisher.publishedEvents[0].AttemptNum)
}

// TestWebhookRetryConsumer_RetriesExhausted 测试重试耗尽后不再发布事件
func TestWebhookRetryConsumer_RetriesExhausted(t *testing.T) {
	var spaceID int64 = 10003

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // 始终失败
	}))
	defer server.Close()

	provider := &mockSecretProvider{secrets: map[int64]string{spaceID: "some-secret"}}
	publisher := &mockPublisher{}
	consumer := NewWebhookRetryConsumer(publisher, provider)

	retryEvent := &entity.WebhookRetryEvent{
		ExptID:     789,
		SpaceID:    spaceID,
		DeliveryID: "dlv_003",
		WebhookURL: server.URL,
		Payload:    `{"event_type":"experiment.terminated"}`,
		AttemptNum: 3, // 已达到 retryDelays 长度，无更多重试
	}
	msgBody, _ := pkgjson.Marshal(retryEvent)
	msg := &mq.MessageExt{
		Message: mq.Message{Body: msgBody},
		MsgID:   "msg_003",
	}

	err := consumer.HandleMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.Len(t, publisher.publishedEvents, 0) // 不再发布重试事件
}

// TestWebhookRetryConsumer_EmptySecret 测试空密钥场景下签名仍一致
func TestWebhookRetryConsumer_EmptySecret(t *testing.T) {
	var spaceID int64 = 10004

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := r.Header.Get("X-CozeLoop-Timestamp")
		nonce := r.Header.Get("X-CozeLoop-Nonce")
		sig := r.Header.Get("X-CozeLoop-Signature")

		// 用空密钥验签
		if !verifySignature("", ts, nonce, sig) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &mockSecretProvider{secrets: map[int64]string{spaceID: ""}} // 空密钥
	publisher := &mockPublisher{}
	consumer := NewWebhookRetryConsumer(publisher, provider)

	retryEvent := &entity.WebhookRetryEvent{
		ExptID:     100,
		SpaceID:    spaceID,
		DeliveryID: "dlv_004",
		WebhookURL: server.URL,
		Payload:    `{"event_type":"experiment.completed"}`,
		AttemptNum: 1,
	}
	msgBody, _ := pkgjson.Marshal(retryEvent)
	msg := &mq.MessageExt{
		Message: mq.Message{Body: msgBody},
		MsgID:   "msg_004",
	}

	err := consumer.HandleMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.Len(t, publisher.publishedEvents, 0) // 请求成功，无重试
}

// TestComputeRetryHMACSHA256 单元测试：验证签名计算结果确定性
func TestComputeRetryHMACSHA256(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		message string
	}{
		{"normal", "my-secret", "1717340000\nabc123\n"},
		{"empty_secret", "", "1717340000\ndef456\n"},
		{"unicode_secret", "密钥", "1717340000\nghi789\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig1 := computeRetryHMACSHA256(tt.secret, tt.message)
			sig2 := computeRetryHMACSHA256(tt.secret, tt.message)

			// 确定性：相同输入产生相同输出
			assert.Equal(t, sig1, sig2)
			// 格式：64 字符 hex 字符串
			assert.Len(t, sig1, 64)
			// 可解码
			_, err := hex.DecodeString(sig1)
			assert.NoError(t, err)
		})
	}
}

// TestVerifySignature_Standalone 独立验证签名函数正确性
func TestVerifySignature_Standalone(t *testing.T) {
	secret := "test-secret"
	timestamp := "1717340000"
	nonce := "abc123def456"

	// 计算签名
	signMessage := timestamp + "\n" + nonce + "\n"
	sig := computeRetryHMACSHA256(secret, signMessage)

	// 验证通过
	assert.True(t, verifySignature(secret, timestamp, nonce, sig))

	// 篡改 timestamp → 验证失败
	assert.False(t, verifySignature(secret, "1717340001", nonce, sig))

	// 篡改 nonce → 验证失败
	assert.False(t, verifySignature(secret, timestamp, "tampered-nonce", sig))

	// 错误密钥 → 验证失败
	assert.False(t, verifySignature("wrong-secret", timestamp, nonce, sig))

	// 篡改签名 → 验证失败
	assert.False(t, verifySignature(secret, timestamp, nonce, "0000000000000000000000000000000000000000000000000000000000000000"))
}

// TestWebhookRetryConsumer_MalformedMessage 测试消息反序列化失败不阻塞
func TestWebhookRetryConsumer_MalformedMessage(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[int64]string{}}
	publisher := &mockPublisher{}
	consumer := NewWebhookRetryConsumer(publisher, provider)

	msg := &mq.MessageExt{
		Message: mq.Message{Body: []byte("not-valid-json")},
		MsgID:   "msg_bad",
	}

	err := consumer.HandleMessage(context.Background(), msg)

	assert.NoError(t, err) // 不重试，返回 nil
	assert.Len(t, publisher.publishedEvents, 0)
}

// TestWebhookRetryConsumer_PayloadIntegrity 验证接收方收到的 payload 与原始一致
func TestWebhookRetryConsumer_PayloadIntegrity(t *testing.T) {
	secret := "integrity-secret"
	var spaceID int64 = 10005

	originalPayload := map[string]interface{}{
		"event_type":    "experiment.completed",
		"experiment_id": "expt_100",
		"delivery_id":   "dlv_100",
		"data": map[string]interface{}{
			"status":   "succeeded",
			"progress": map[string]interface{}{"total": 10, "succeeded": 10},
		},
	}
	payloadBytes, _ := json.Marshal(originalPayload)

	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := r.Header.Get("X-CozeLoop-Timestamp")
		nonce := r.Header.Get("X-CozeLoop-Nonce")
		sig := r.Header.Get("X-CozeLoop-Signature")

		if !verifySignature(secret, ts, nonce, sig) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &mockSecretProvider{secrets: map[int64]string{spaceID: secret}}
	publisher := &mockPublisher{}
	consumer := NewWebhookRetryConsumer(publisher, provider)

	retryEvent := &entity.WebhookRetryEvent{
		ExptID:     100,
		SpaceID:    spaceID,
		DeliveryID: "dlv_100",
		WebhookURL: server.URL,
		Payload:    string(payloadBytes),
		AttemptNum: 1,
	}
	msgBody, _ := pkgjson.Marshal(retryEvent)
	msg := &mq.MessageExt{
		Message: mq.Message{Body: msgBody},
		MsgID:   "msg_integrity",
	}

	err := consumer.HandleMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.NotNil(t, receivedPayload)
	assert.Equal(t, "experiment.completed", receivedPayload["event_type"])
	assert.Equal(t, "expt_100", receivedPayload["experiment_id"])
}

// TestWebhookRetryConsumer_RealCallbackVerification 使用真实回调数据验证签名
func TestWebhookRetryConsumer_RealCallbackVerification(t *testing.T) {
	secret := "cede3191624f4abba32cfe3cd04da2b3"
	timestamp := "1780424459"
	nonce := "277a3c2310edffc44ec499ecaba729a9"
	gotSignature := "35dc1cd19bc3cf4f8147ed48c0dc4b71a3f055b2ba3c61dc80598ca95861b041"

	// 验签：HMAC-SHA256(secret, timestamp + "\n" + nonce + "\n")
	result := verifySignature(secret, timestamp, nonce, gotSignature)
	assert.True(t, result, "使用真实回调数据验签应该通过")

	// 额外验证：手动计算一遍确认
	signMessage := timestamp + "\n" + nonce + "\n"
	expectedSig := computeRetryHMACSHA256(secret, signMessage)
	assert.Equal(t, gotSignature, expectedSig, "手动计算的签名应与回调中携带的签名一致")
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
