// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/require"
)

func TestTraceHubServiceImpl_getObjListWithTaskFromCache_Fallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	impl := &TraceHubServiceImpl{
		localCache: NewLocalCache(),
	}

	cache := impl.localCache.LoadTaskCache(ctx)
	require.Nil(t, cache.WorkspaceIDs)
	require.Nil(t, cache.BotIDs)
	require.Nil(t, cache.Tasks)
}

func TestTraceHubServiceImpl_getObjListWithTaskFromCache_FromCache(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{
		taskRepo:   mockRepo,
		localCache: NewLocalCache(),
	}

	expected := TaskCacheInfo{
		WorkspaceIDs: []string{"space-2"},
		BotIDs:       []string{"bot-2"},
		Tasks:        []*entity.ObservabilityTask{{}},
	}
	impl.localCache.taskCache.Store("ObjListWithTask", expected)

	cache := impl.localCache.LoadTaskCache(context.Background())
	require.Equal(t, expected.WorkspaceIDs, cache.WorkspaceIDs)
	require.Equal(t, expected.BotIDs, cache.BotIDs)
	require.Equal(t, len(expected.Tasks), len(cache.Tasks))
}

func TestTraceHubServiceImpl_getObjListWithTaskFromCache_TypeMismatch(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{
		localCache: NewLocalCache(),
	}

	impl.localCache.taskCache.Store("ObjListWithTask", "invalid")

	cache := impl.localCache.LoadTaskCache(context.Background())
	require.Nil(t, cache.WorkspaceIDs)
	require.Nil(t, cache.BotIDs)
	require.Nil(t, cache.Tasks)
}

func TestTraceHubServiceImpl_applySampling(t *testing.T) {
	t.Parallel()

	spans := []*loop_span.Span{{SpanID: "1"}, {SpanID: "2"}, {SpanID: "3"}}
	impl := &TraceHubServiceImpl{}

	fullRate := &spanSubscriber{t: &entity.ObservabilityTask{Sampler: &entity.Sampler{SampleRate: 1.0}}}
	zeroRate := &spanSubscriber{t: &entity.ObservabilityTask{Sampler: &entity.Sampler{SampleRate: 0}}}
	halfRate := &spanSubscriber{t: &entity.ObservabilityTask{Sampler: &entity.Sampler{SampleRate: 0.5}}}

	require.Len(t, impl.applySampling(spans, fullRate), len(spans))
	require.Nil(t, impl.applySampling(spans, zeroRate))
	require.Len(t, impl.applySampling(spans, halfRate), 1)
}

func TestTraceHubServiceImpl_sendBackfillMessage(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	err := impl.sendBackfillMessage(context.Background(), &entity.BackFillEvent{})
	require.Error(t, err)

	fake := &fakeBackfillProducer{}
	impl.backfillProducer = fake

	evt := &entity.BackFillEvent{TaskID: 1, SpaceID: 2}
	require.NoError(t, impl.sendBackfillMessage(context.Background(), evt))
	require.Equal(t, evt, fake.event)
}

type fakeBackfillProducer struct {
	event *entity.BackFillEvent
}

func (f *fakeBackfillProducer) SendBackfill(_ context.Context, event *entity.BackFillEvent) error {
	f.event = event
	return nil
}
