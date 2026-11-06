// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type publicationPrefixFixture struct {
	scheduler *ExptSchedulerImpl
	service   *svcmocks.MockExptResultPublicationService
	results   *svcmocks.MockExptResultService
	mode      *entitymocks.MockExptSchedulerMode
	publisher *eventmocks.MockExptEventPublisher
	event     *entity.ExptScheduleEvent
	expt      *entity.Experiment
}

func newPublicationPrefixFixture(t *testing.T) *publicationPrefixFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &publicationPrefixFixture{
		service:   svcmocks.NewMockExptResultPublicationService(ctrl),
		results:   svcmocks.NewMockExptResultService(ctrl),
		mode:      entitymocks.NewMockExptSchedulerMode(ctrl),
		publisher: eventmocks.NewMockExptEventPublisher(ctrl),
		event:     &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2},
		expt:      publishUnitExperiment(),
	}
	f.scheduler = &ExptSchedulerImpl{ResultSvc: f.results, Publisher: f.publisher}
	return f
}

func publicationPrefixItem(id int64) *entity.ExptEvalItem {
	source := publishUnitSource()
	source.Source.Current.ItemID = id
	source.Source.CanonicalTurns[0].ItemID = id
	source.Source.Turns[0].ItemID = id
	source.Item.ItemID = id
	source.ItemLog.ItemID = id
	return &entity.ExptEvalItem{ItemID: id, RequestID: "req", State: entity.ItemRunState_Success, Publication: source}
}

func TestRecordCurrentItemResultsRefreshesCommittedPrefix(t *testing.T) {
	for _, stopAt := range []string{"source_error", "prepare_error", "publish_error", "lock_busy", "not_applied"} {
		t.Run(stopAt, func(t *testing.T) {
			f := newPublicationPrefixFixture(t)
			first, second := publicationPrefixItem(9), publicationPrefixItem(10)
			publication := &entity.ExptItemResultPublication{Source: first.Publication}
			f.service.EXPECT().PrepareItemResultPublication(gomock.Any(), first.Publication, f.expt).Return(publication, nil)
			f.service.EXPECT().PublishPreparedItemResult(gomock.Any(), publication, gomock.Any()).Return(true, nil)
			f.mode.EXPECT().PublishResult(gomock.Any(), publication.Refs, f.event).Return(nil)
			fault := errors.New("suffix publication failed")
			switch stopAt {
			case "source_error":
				second.Publication = nil
				f.service.EXPECT().GetItemResultPublishSource(gomock.Any(), int64(3), int64(1), int64(2), int64(10)).Return(nil, fault)
			case "prepare_error":
				f.service.EXPECT().PrepareItemResultPublication(gomock.Any(), second.Publication, f.expt).Return(nil, fault)
			default:
				suffix := &entity.ExptItemResultPublication{Source: second.Publication}
				f.service.EXPECT().PrepareItemResultPublication(gomock.Any(), second.Publication, f.expt).Return(suffix, nil)
				switch stopAt {
				case "lock_busy":
					fault = errno.NewMQRetryErr("publication item lock busy")
				case "not_applied":
					fault = nil
				}
				f.service.EXPECT().PublishPreparedItemResult(gomock.Any(), suffix, gomock.Any()).Return(false, fault)
			}
			var refreshed []int64
			f.results.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).DoAndReturn(func(ctx context.Context, _, _ int64, ids []int64) error {
				require.NoError(t, ctx.Err())
				refreshed = append([]int64(nil), ids...)
				return nil
			})
			var checked []int64
			f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptTurnResultFilterEvent, delay *time.Duration) error {
				require.Equal(t, int64(3), event.SpaceID)
				require.Equal(t, int64(1), event.ExperimentID)
				require.Equal(t, entity.UpsertExptTurnResultFilterTypeCheck, *event.FilterType)
				require.Equal(t, int32(0), *event.RetryTimes)
				require.Equal(t, 10*time.Second, *delay)
				checked = append([]int64(nil), event.ItemID...)
				return nil
			})
			err := f.scheduler.recordCurrentItemResults(context.Background(), f.event, []*entity.ExptEvalItem{first, second}, f.mode, f.expt, f.service)
			if fault != nil {
				require.ErrorIs(t, err, fault)
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, []int64{9}, refreshed)
			require.Equal(t, []int64{9}, checked)
		})
	}
}

type publicationPrefixContextKey struct{}

func TestRecordCurrentItemResultsCancelledPrefixQueuesRepair(t *testing.T) {
	for _, cancelAt := range []string{"after_commit", "publish_result", "refresh"} {
		t.Run(cancelAt, func(t *testing.T) {
			f := newPublicationPrefixFixture(t)
			item := publicationPrefixItem(9)
			publication := &entity.ExptItemResultPublication{Source: item.Publication}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), publicationPrefixContextKey{}, "request-context"))
			defer cancel()
			f.service.EXPECT().PrepareItemResultPublication(gomock.Any(), item.Publication, f.expt).Return(publication, nil)
			f.service.EXPECT().PublishPreparedItemResult(gomock.Any(), publication, gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptItemResultPublication, func(context.Context, *entity.ExptItemResultPublishSource) error) (bool, error) {
				if cancelAt == "after_commit" {
					cancel()
				}
				return true, nil
			})
			if cancelAt != "after_commit" {
				f.mode.EXPECT().PublishResult(gomock.Any(), publication.Refs, f.event).DoAndReturn(func(context.Context, []*entity.ExptTurnEvaluatorResultRef, *entity.ExptScheduleEvent) error {
					if cancelAt == "publish_result" {
						cancel()
					}
					return nil
				})
			}
			if cancelAt == "refresh" {
				f.results.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).DoAndReturn(func(context.Context, int64, int64, []int64) error {
					cancel()
					return context.Canceled
				})
			}
			var queued []entity.UpsertExptTurnResultFilterType
			f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, event *entity.ExptTurnResultFilterEvent, _ *time.Duration) error {
				require.NoError(t, ctx.Err())
				require.Equal(t, "request-context", ctx.Value(publicationPrefixContextKey{}))
				deadline, ok := ctx.Deadline()
				require.True(t, ok, "post-commit repair must have a bounded lifetime")
				require.Positive(t, time.Until(deadline))
				require.LessOrEqual(t, time.Until(deadline), time.Minute)
				require.Equal(t, []int64{9}, event.ItemID)
				queued = append(queued, *event.FilterType)
				return nil
			}).Times(2)
			err := f.scheduler.recordCurrentItemResults(ctx, f.event, []*entity.ExptEvalItem{item}, f.mode, f.expt, f.service)
			require.ErrorIs(t, err, context.Canceled)
			require.True(t, errno.NeedMQRetry(err))
			require.Equal(t, []entity.UpsertExptTurnResultFilterType{entity.UpsertExptTurnResultFilterTypeAuto, entity.UpsertExptTurnResultFilterTypeCheck}, queued)
		})
	}
}

func TestRecordCurrentItemResultsRetainsPrefixAndRepairErrors(t *testing.T) {
	for _, failAt := range []string{"refresh_auto", "check", "refresh_repaired"} {
		t.Run(failAt, func(t *testing.T) {
			f := newPublicationPrefixFixture(t)
			first, second := publicationPrefixItem(9), publicationPrefixItem(10)
			publication := &entity.ExptItemResultPublication{Source: first.Publication}
			f.service.EXPECT().PrepareItemResultPublication(gomock.Any(), first.Publication, f.expt).Return(publication, nil)
			f.service.EXPECT().PublishPreparedItemResult(gomock.Any(), publication, gomock.Any()).Return(true, nil)
			f.mode.EXPECT().PublishResult(gomock.Any(), publication.Refs, f.event).Return(nil)
			publicationErr := errno.NewMQRetryErr("publication item lock busy")
			f.service.EXPECT().PrepareItemResultPublication(gomock.Any(), second.Publication, f.expt).Return(nil, publicationErr)
			refreshErr, queueErr := errors.New("refresh failed"), errors.New("queue failed")
			if failAt == "check" {
				refreshErr = nil
			}
			f.results.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).Return(refreshErr)
			var queued []entity.UpsertExptTurnResultFilterType
			count := 2
			if refreshErr == nil {
				count = 1
			}
			f.publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptTurnResultFilterEvent, _ *time.Duration) error {
				require.Equal(t, []int64{9}, event.ItemID)
				queued = append(queued, *event.FilterType)
				if failAt == "refresh_auto" && *event.FilterType == entity.UpsertExptTurnResultFilterTypeAuto || failAt == "check" && *event.FilterType == entity.UpsertExptTurnResultFilterTypeCheck {
					return queueErr
				}
				return nil
			}).Times(count)
			err := f.scheduler.recordCurrentItemResults(context.Background(), f.event, []*entity.ExptEvalItem{first, second}, f.mode, f.expt, f.service)
			require.ErrorIs(t, err, publicationErr)
			require.True(t, errno.NeedMQRetry(err))
			if failAt != "refresh_repaired" {
				require.ErrorIs(t, err, queueErr)
			}
			if failAt == "refresh_auto" {
				require.ErrorIs(t, err, refreshErr)
			}
			if refreshErr == nil {
				require.Equal(t, []entity.UpsertExptTurnResultFilterType{entity.UpsertExptTurnResultFilterTypeCheck}, queued)
			} else {
				require.Equal(t, []entity.UpsertExptTurnResultFilterType{entity.UpsertExptTurnResultFilterTypeAuto, entity.UpsertExptTurnResultFilterTypeCheck}, queued)
			}
		})
	}
}
