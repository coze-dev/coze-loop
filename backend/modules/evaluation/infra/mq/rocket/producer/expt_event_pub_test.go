// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
)

func TestBatchSendWithRetry(t *testing.T) {
	tests := []struct {
		name          string
		failCount     int // SendBatch连续失败次数，之后返回成功; -1表示一直失败
		ctxCancel     bool
		wantErr       bool
		wantMinCalls  int
	}{
		{
			name:         "success on first attempt",
			failCount:    0,
			wantErr:      false,
			wantMinCalls: 1,
		},
		{
			name:         "success after transient failures",
			failCount:    2,
			wantErr:      false,
			wantMinCalls: 3,
		},
		{
			name:         "all attempts fail within timeout",
			failCount:    -1,
			wantErr:      true,
			wantMinCalls: 2,
		},
		{
			name:         "context canceled - stops retry",
			failCount:    -1,
			ctxCancel:    true,
			wantErr:      true,
			wantMinCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProducer := mqmocks.NewMockIProducer(ctrl)
			var callCount atomic.Int32
			mockProducer.EXPECT().SendBatch(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, msgs []*mq.Message) (mq.SendResponse, error) {
					idx := int(callCount.Add(1))
					if tt.failCount == -1 || idx <= tt.failCount {
						return mq.SendResponse{}, errors.New("connection reset")
					}
					return mq.SendResponse{MessageID: "msg1", Offset: 1}, nil
				},
			).AnyTimes()

			pub := &exptEventPublisher{
				producers: map[string]*producer{
					rocket.ExptScheduleEventRMQKey: {
						cfg: rocket.RMQConf{Topic: "test_topic"},
						p:   mockProducer,
					},
				},
			}

			ctx := context.Background()
			if tt.ctxCancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
			err := pub.PublishExptScheduleEvent(ctx, event, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "send batch message fail")
			} else {
				assert.NoError(t, err)
			}
			assert.GreaterOrEqual(t, int(callCount.Load()), tt.wantMinCalls)
		})
	}
}
