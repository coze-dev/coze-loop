// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
)

type failedEventJSON struct{ cause error }

func (f failedEventJSON) MarshalJSON() ([]byte, error) { return nil, f.cause }

func TestPublishScheduleMissingProducerNotSent(t *testing.T) {
	pub := &exptEventPublisher{producers: map[string]*producer{}}
	err := pub.PublishExptScheduleEvent(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2}, nil)
	require.ErrorIs(t, err, events.ErrPublishNotSent)
	require.Contains(t, err.Error(), rocket.ExptScheduleEventRMQKey)
	require.ErrorIs(t, fmt.Errorf("dispatch: %w", err), events.ErrPublishNotSent)
}

func TestBatchSerializationNotSent(t *testing.T) {
	for _, badIndex := range []int{0, 1} {
		t.Run(fmt.Sprint(badIndex), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mqProducer := mqmocks.NewMockIProducer(ctrl)
			pub := &exptEventPublisher{producers: map[string]*producer{
				rocket.ExptScheduleEventRMQKey: {cfg: rocket.RMQConf{Topic: "test"}, p: mqProducer},
			}}
			cause := errors.New("event encoding failed")
			payloads := []any{&entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2}, failedEventJSON{cause}}
			if badIndex == 0 {
				payloads[0], payloads[1] = payloads[1], payloads[0]
			}
			mqProducer.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Times(0)
			delay := time.Second
			err := pub.batchSendWithTagAndKeys(context.Background(), rocket.ExptScheduleEventRMQKey, "tag", payloads, &delay, []string{"key"})
			require.ErrorIs(t, err, events.ErrPublishNotSent)
			require.ErrorIs(t, err, cause, "retain the original serialization cause")
		})
	}
}

func TestPublishScheduleSendOutcomeNotMarked(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"success", nil},
		{"unknown", errors.New("acknowledgement lost")},
		{"canceled", context.Canceled},
		{"deadline", context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mqProducer := mqmocks.NewMockIProducer(ctrl)
			pub := &exptEventPublisher{producers: map[string]*producer{
				rocket.ExptScheduleEventRMQKey: {cfg: rocket.RMQConf{Topic: "test"}, p: mqProducer},
			}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mqProducer.EXPECT().SendBatch(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, msgs []*mq.Message) (mq.SendResponse, error) {
					require.Len(t, msgs, 1)
					cancel() // Stop retries after an actual send attempt.
					return mq.SendResponse{}, tc.err
				})
			err := pub.PublishExptScheduleEvent(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2}, nil)
			require.ErrorIs(t, err, tc.err)
			require.False(t, errors.Is(err, events.ErrPublishNotSent))
		})
	}
}
