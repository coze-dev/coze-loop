// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func filterBoundaryExpectedProjection() *entity.ExptTurnResultFilterEntity {
	weight := 0.5
	return &entity.ExptTurnResultFilterEntity{
		SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: 0, ItemIdx: 0, ItemVersionID: 32,
		Status: entity.ItemRunState_Success, EvalSetID: 30, EvalSetVersionID: 31,
		CreatedDate:     time.Date(2025, 2, 3, 0, 0, 0, 0, time.UTC),
		EvalTargetData:  map[string]string{"actual_output": "", "custom": "preview"},
		EvaluatorScore:  map[string]float64{"key11": 0, "key12": 1},
		AnnotationFloat: map[string]float64{"key1": 0}, AnnotationBool: map[string]bool{},
		AnnotationString:       map[string]string{"81": "", "82": "83"},
		EvalTargetMetrics:      map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501},
		EvaluatorWeightedScore: &weight,
	}
}

func TestFilterCheckBoundsRetriesAndKeepsErrorsVisible(t *testing.T) {
	for _, mode := range []string{"matching", "repair", "retry_exhausted", "future", "publisher_missing", "publish_failure", "read_failure", "malformed_index", "source_failure", "cancel_after_snapshot"} {
		t.Run(mode, func(t *testing.T) {
			f := newProjectionFixture(t)
			svc := f.service()
			ctrl := gomock.NewController(t)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			svc.publisher = publisher
			stamp := uint64(time.Now().Add(-time.Second).UnixMilli())
			legacy, v2 := filterBoundaryExpectedProjection(), filterBoundaryExpectedProjection()
			index := &filterBoundaryIndex{
				IExptTurnResultFilterRepo: svc.exptTurnResultFilterRepo,
				plan:                      &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}, ExptIDs: []int64{2}, ConfigHash: "frozen"},
				rows: map[entity.ExptFilterTarget][]*entity.ExptFilterRow{
					entity.ExptFilterTargetLegacy: {{Filter: legacy}},
					entity.ExptFilterTargetV2:     {{Filter: v2, WriteTimestampMS: &stamp}},
				},
			}
			svc.exptTurnResultFilterRepo = index
			fault := errors.New("index boundary unavailable")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			retry := int32(0)
			switch mode {
			case "repair", "retry_exhausted", "publisher_missing", "publish_failure":
				v2.EvalTargetData["actual_output"] = "stale"
			case "future":
				stamp = uint64(time.Now().Add(time.Hour).UnixMilli())
			case "read_failure":
				index.readErrTarget, index.readErr = entity.ExptFilterTargetV2, fault
			case "malformed_index":
				index.rows[entity.ExptFilterTargetV2][0].WriteTimestampMS = nil
			case "source_failure":
				f.fail = "targets"
			case "cancel_after_snapshot":
				index.snapshotHook = cancel
			}
			if mode == "retry_exhausted" {
				retry = 3
			}
			if mode == "future" || mode == "retry_exhausted" {
				metrics := metricsmocks.NewMockExptMetric(ctrl)
				metrics.EXPECT().EmitExptTurnResultFilterCheck(int64(1), false, false, true, false)
				svc.Metric = metrics
			}
			if mode == "publisher_missing" {
				svc.publisher = nil
			}
			if mode == "repair" || mode == "publish_failure" {
				publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Nil()).DoAndReturn(func(_ context.Context, event *entity.ExptTurnResultFilterEvent, _ *time.Duration) error {
					require.Equal(t, int64(1), event.SpaceID)
					require.Equal(t, int64(2), event.ExperimentID)
					require.Equal(t, []int64{3}, event.ItemID)
					require.Equal(t, entity.UpsertExptTurnResultFilterTypeAuto, *event.FilterType)
					require.Equal(t, int32(1), *event.RetryTimes)
					if mode == "publish_failure" {
						return fault
					}
					return nil
				})
			}
			before := projectionSourceDigest(t, f)
			err := svc.CompareExptTurnResultFilters(ctx, 1, 2, []int64{3}, retry)
			require.Equal(t, before, projectionSourceDigest(t, f))
			switch mode {
			case "publisher_missing":
				require.True(t, errno.NeedMQRetry(err))
			case "publish_failure", "read_failure":
				require.ErrorIs(t, err, fault)
			case "malformed_index":
				require.ErrorContains(t, err, "missing write timestamp")
			case "source_failure":
				require.ErrorContains(t, err, "stage=targets")
				require.Empty(t, index.reads)
			case "cancel_after_snapshot":
				require.ErrorIs(t, err, context.Canceled)
				require.True(t, errno.NeedMQRetry(err))
				require.Zero(t, f.pages)
				require.Empty(t, index.reads)
			default:
				require.NoError(t, err)
			}
			if mode != "source_failure" && mode != "cancel_after_snapshot" {
				require.Equal(t, []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}, index.reads)
			}
			require.Zero(t, index.saveCalls, "CHECK must not write the index inline, including with nil rows")
			require.Nil(t, index.savedRows)
		})
	}
}
