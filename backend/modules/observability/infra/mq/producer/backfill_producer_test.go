// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
)

type stubProducer struct {
	lastMessage *mq.Message
}

func (s *stubProducer) Start() error { return nil }
func (s *stubProducer) Close() error { return nil }
func (s *stubProducer) Send(ctx context.Context, message *mq.Message) (mq.SendResponse, error) {
	s.lastMessage = message
	return mq.SendResponse{MessageID: "mid"}, nil
}

func (s *stubProducer) SendBatch(ctx context.Context, messages []*mq.Message) (mq.SendResponse, error) {
	return mq.SendResponse{}, nil
}

func (s *stubProducer) SendAsync(ctx context.Context, callback mq.AsyncSendCallback, message *mq.Message) error {
	return nil
}

func TestBackfillProducer_SendBackfill_DelayExponentialAndCap(t *testing.T) {
	p := &BackfillProducerImpl{topic: "t", mqProducer: &stubProducer{}}

	// retry=0 → base delay (10s)
	err := p.SendBackfill(context.Background(), &entity.BackFillEvent{SpaceID: 1, TaskID: 2, Retry: 0})
	assert.NoError(t, err)
	msg := p.mqProducer.(*stubProducer).lastMessage
	assert.Equal(t, 10*time.Second, msg.DeferDuration)
	assert.Equal(t, "backfill", msg.Tag)

	// retry=3 → 10s * (1<<3) = 80s
	err = p.SendBackfill(context.Background(), &entity.BackFillEvent{SpaceID: 1, TaskID: 2, Retry: 3})
	assert.NoError(t, err)
	msg = p.mqProducer.(*stubProducer).lastMessage
	assert.Equal(t, 80*time.Second, msg.DeferDuration)

	// retry large (29) → capped at 10m (avoid int64 overflow)
	err = p.SendBackfill(context.Background(), &entity.BackFillEvent{SpaceID: 1, TaskID: 2, Retry: 29})
	assert.NoError(t, err)
	msg = p.mqProducer.(*stubProducer).lastMessage
	assert.Equal(t, 10*time.Minute, msg.DeferDuration)
}
