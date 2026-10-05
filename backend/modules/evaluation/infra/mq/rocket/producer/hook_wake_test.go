// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/mq/rocketmq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func hookWakeFixture() entity.HookWakeEvent {
	return entity.HookWakeEvent{Run: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 9007199254740993}, OperationID: "hook_42", ExecutionScope: "ppe_hooks"}
}

func TestHookWakePublisherMinimalMessage(t *testing.T) {
	p := mqmocks.NewMockIProducer(gomock.NewController(t))
	pub, err := NewHookWakePublisher(p, "hook-wake", time.Second)
	require.NoError(t, err)
	p.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, msg *mq.Message) (mq.SendResponse, error) {
		require.Equal(t, "hook-wake", msg.Topic)
		require.Equal(t, "lifecycle_hook_wake", msg.Tag)
		require.JSONEq(t, `{"workspace_id":1,"experiment_id":2,"run_id":9007199254740993,"operation_id":"hook_42","execution_scope":"ppe_hooks"}`, string(msg.Body))
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(msg.Body, &fields))
		require.Equal(t, "9007199254740993", string(fields["run_id"]))
		require.Empty(t, msg.Properties)
		require.Empty(t, msg.Keys)
		require.Empty(t, msg.PartitionKey)
		require.Zero(t, msg.DeferDuration)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.WithinDuration(t, time.Now().Add(time.Second), deadline, 200*time.Millisecond)
		return mq.SendResponse{MessageID: "broker-id"}, nil
	})
	require.NoError(t, pub.PublishWake(context.Background(), hookWakeFixture()))
}

func TestHookWakePublisherFailureIsVisibleAndRedacted(t *testing.T) {
	p := mqmocks.NewMockIProducer(gomock.NewController(t))
	pub, err := NewHookWakePublisher(p, "hook-wake", time.Second)
	require.NoError(t, err)
	p.EXPECT().Send(gomock.Any(), gomock.Any()).Return(mq.SendResponse{}, errors.New("secret@example.com credential=raw"))
	err = pub.PublishWake(context.Background(), hookWakeFixture())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret@example.com")
	require.Nil(t, errors.Unwrap(err))
}

func TestHookWakePublisherDeadlineAndCancellation(t *testing.T) {
	for _, lateSuccess := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout_error", true: "late_success"}[lateSuccess], func(t *testing.T) {
			p := mqmocks.NewMockIProducer(gomock.NewController(t))
			pub, err := NewHookWakePublisher(p, "hook-wake", 10*time.Millisecond)
			require.NoError(t, err)
			p.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ *mq.Message) (mq.SendResponse, error) {
				<-ctx.Done()
				if lateSuccess {
					return mq.SendResponse{}, nil
				}
				return mq.SendResponse{}, ctx.Err()
			})
			require.ErrorIs(t, pub.PublishWake(context.Background(), hookWakeFixture()), context.DeadlineExceeded)
		})
	}
	p := mqmocks.NewMockIProducer(gomock.NewController(t))
	pub, err := NewHookWakePublisher(p, "hook-wake", time.Second)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, pub.PublishWake(ctx, hookWakeFixture()), context.Canceled)
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	require.ErrorIs(t, pub.PublishWake(ctx, hookWakeFixture()), context.DeadlineExceeded)
}

func TestHookWakePublisherInvalidEventNeverSends(t *testing.T) {
	p := mqmocks.NewMockIProducer(gomock.NewController(t))
	pub, err := NewHookWakePublisher(p, "hook-wake", time.Second)
	require.NoError(t, err)
	require.ErrorIs(t, pub.PublishWake(context.Background(), entity.HookWakeEvent{}), entity.ErrInvalidHookWakeEvent)
}

func TestHookWakePublisherConstructionRejectsMissingDependency(t *testing.T) {
	p := mqmocks.NewMockIProducer(gomock.NewController(t))
	for _, tc := range []struct {
		p       mq.IProducer
		topic   string
		timeout time.Duration
	}{{nil, "hook-wake", time.Second}, {p, "", time.Second}, {p, " hook-wake", time.Second}, {p, "hook-wake", 0}, {p, "hook-wake", -1}} {
		pub, err := NewHookWakePublisher(tc.p, tc.topic, tc.timeout)
		require.Error(t, err)
		require.Nil(t, pub)
	}
}

func TestHookWakePublisherRejectsTypedNilProducer(t *testing.T) {
	var dependency *rocketmq.Producer
	pub, err := NewHookWakePublisher(dependency, "hook-wake", time.Second)
	require.Error(t, err)
	require.Nil(t, pub)
}

type hookWakeValueProducer struct{ mq.IProducer }

func TestHookWakePublisherAcceptsValueProducer(t *testing.T) {
	dependency := mqmocks.NewMockIProducer(gomock.NewController(t))
	pub, err := NewHookWakePublisher(hookWakeValueProducer{dependency}, "hook-wake", time.Second)
	require.NoError(t, err)
	dependency.EXPECT().Send(gomock.Any(), gomock.Any()).Return(mq.SendResponse{MessageID: "value-producer"}, nil)
	require.NoError(t, pub.PublishWake(context.Background(), hookWakeFixture()))
}
