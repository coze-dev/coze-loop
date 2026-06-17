// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
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
		name         string
		sendResults  []struct {
			resp mq.SendResponse
			err  error
		}
		ctxCancel bool
		wantErr   bool
	}{
		{
			name: "success on first attempt",
			sendResults: []struct {
				resp mq.SendResponse
				err  error
			}{
				{resp: mq.SendResponse{MessageID: "msg1", Offset: 1}, err: nil},
			},
			wantErr: false,
		},
		{
			name: "success on second attempt after transient failure",
			sendResults: []struct {
				resp mq.SendResponse
				err  error
			}{
				{resp: mq.SendResponse{}, err: errors.New("connection reset")},
				{resp: mq.SendResponse{MessageID: "msg1", Offset: 1}, err: nil},
			},
			wantErr: false,
		},
		{
			name: "success on third attempt",
			sendResults: []struct {
				resp mq.SendResponse
				err  error
			}{
				{resp: mq.SendResponse{}, err: errors.New("timeout")},
				{resp: mq.SendResponse{}, err: errors.New("timeout")},
				{resp: mq.SendResponse{MessageID: "msg1", Offset: 1}, err: nil},
			},
			wantErr: false,
		},
		{
			name: "all 3 attempts fail",
			sendResults: []struct {
				resp mq.SendResponse
				err  error
			}{
				{resp: mq.SendResponse{}, err: errors.New("timeout")},
				{resp: mq.SendResponse{}, err: errors.New("timeout")},
				{resp: mq.SendResponse{}, err: errors.New("timeout")},
			},
			wantErr: true,
		},
		{
			name: "context canceled - no retry",
			sendResults: []struct {
				resp mq.SendResponse
				err  error
			}{
				{resp: mq.SendResponse{}, err: errors.New("context canceled")},
			},
			ctxCancel: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProducer := mqmocks.NewMockIProducer(ctrl)
			callIdx := 0
			mockProducer.EXPECT().SendBatch(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, msgs []*mq.Message) (mq.SendResponse, error) {
					if callIdx >= len(tt.sendResults) {
						t.Fatal("unexpected extra SendBatch call")
					}
					result := tt.sendResults[callIdx]
					callIdx++
					return result.resp, result.err
				},
			).Times(len(tt.sendResults))

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
			assert.Equal(t, len(tt.sendResults), callIdx)
		})
	}
}
