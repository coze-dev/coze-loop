// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
}

type WebhookRetryConsumer struct {
	publisher      events.ExptEventPublisher
	secretProvider service.IWebhookSecretProvider
	httpClient     *http.Client
}

func NewWebhookRetryConsumer(publisher events.ExptEventPublisher, secretProvider service.IWebhookSecretProvider) mq.IConsumerHandler {
	return &WebhookRetryConsumer{
		publisher:      publisher,
		secretProvider: secretProvider,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *WebhookRetryConsumer) HandleMessage(ctx context.Context, msg *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "[WebhookRetryConsumer] consume message fail, msg_id: %v, err: %v", msg.MsgID, err)
		}
	}()

	body := msg.Body
	event := &entity.WebhookRetryEvent{}
	if err := json.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "[WebhookRetryConsumer] unmarshal fail, raw: %v, err: %s", conv.UnsafeBytesToString(body), err)
		return nil // 反序列化失败不重试
	}

	logs.CtxInfo(ctx, "[WebhookRetryConsumer] retry attempt %d, delivery_id: %v, url: %v, expt_id: %v",
		event.AttemptNum, event.DeliveryID, event.WebhookURL, event.ExptID)

	// 重新签名（每次重试 timestamp/nonce 变化）
	var secret string
	if c.secretProvider != nil {
		secret, _ = c.secretProvider.GetSecret(ctx, event.SpaceID)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := generateRetryNonce()
	signature := computeRetryHMACSHA256(secret, timestamp+"\n"+nonce+"\n")

	// HTTP POST
	if err := c.doPost(ctx, event.WebhookURL, []byte(event.Payload), timestamp, nonce, signature); err != nil {
		logs.CtxWarn(ctx, "[WebhookRetryConsumer] retry attempt %d failed, url: %v, err: %v", event.AttemptNum, event.WebhookURL, err)

		// 如果还有重试机会，发布下一次重试
		if event.AttemptNum < len(retryDelays) {
			nextEvent := &entity.WebhookRetryEvent{
				ExptID:     event.ExptID,
				SpaceID:    event.SpaceID,
				DeliveryID: event.DeliveryID,
				WebhookURL: event.WebhookURL,
				Payload:    event.Payload,
				AttemptNum: event.AttemptNum + 1,
			}
			delay := retryDelays[event.AttemptNum] // AttemptNum 从 1 开始，索引 [1] = 5min, [2] = 30min
			if pubErr := c.publisher.PublishExptWebhookNotifyEvent(ctx, nextEvent, &delay); pubErr != nil {
				logs.CtxError(ctx, "[WebhookRetryConsumer] publish next retry failed, attempt: %d, err: %v", event.AttemptNum+1, pubErr)
			}
		} else {
			logs.CtxError(ctx, "[WebhookRetryConsumer] all retries exhausted, delivery_id: %v, url: %v, expt_id: %v",
				event.DeliveryID, event.WebhookURL, event.ExptID)
		}
		return nil // 不让 MQ 自动重试，由我们自行管理
	}

	logs.CtxInfo(ctx, "[WebhookRetryConsumer] retry attempt %d succeeded, delivery_id: %v, url: %v",
		event.AttemptNum, event.DeliveryID, event.WebhookURL)
	return nil
}

func (c *WebhookRetryConsumer) doPost(ctx context.Context, url string, body []byte, timestamp, nonce, signature string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CozeLoop-Timestamp", timestamp)
	req.Header.Set("X-CozeLoop-Nonce", nonce)
	req.Header.Set("X-CozeLoop-Signature", signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}

func computeRetryHMACSHA256(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func generateRetryNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
