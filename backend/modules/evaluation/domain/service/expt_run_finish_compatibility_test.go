// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRunFinishCheckDisposition(t *testing.T) {
	for _, kind := range []string{"pending", "nil_result", "already", "noncurrent", "finished", "error"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil)
			probe := &checkFinishProbe{IExptManager: manager, result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishAlreadyFinished}}
			switch kind {
			case "pending":
				probe.result.Disposition = entity.ExptRunFinishPending
			case "nil_result":
				probe.result = nil
			case "noncurrent":
				probe.result.Disposition = entity.ExptRunFinishNonCurrent
			case "finished":
				probe.result.Disposition = entity.ExptRunFinishFinished
			case "error":
				probe.err = errno.NewMQRetryErr("finish dependency failed")
			}
			scheduler := &ExptSchedulerImpl{Manager: probe}
			err := scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
				t.Fatal("finished event reached execution")
				return nil
			})(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit})
			if kind == "pending" || kind == "nil_result" || kind == "error" {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type checkFinishProbe struct {
	IExptManager
	result *entity.ExptRunFinishResult
	err    error
}

func (p *checkFinishProbe) FinishCurrentRun(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	return p.result, p.err
}
func (p *checkFinishProbe) FinishCurrentRunWithHeldTickLock(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	return p.result, p.err
}

type cleanupTestIdem struct {
	seen            bool
	readErr, setErr error
	keys            []string
	ttl             time.Duration
}

func (i *cleanupTestIdem) Exist(_ context.Context, key string) (bool, error) {
	i.keys = append(i.keys, key)
	return i.seen, i.readErr
}
func (i *cleanupTestIdem) Set(_ context.Context, key string, ttl time.Duration) error {
	i.keys = append(i.keys, key)
	i.ttl = ttl
	if i.setErr != nil {
		return i.setErr
	}
	i.seen = true
	return nil
}
func (i *cleanupTestIdem) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if i.seen {
		return false, nil
	}
	return true, i.Set(ctx, key, ttl)
}
func (i *cleanupTestIdem) Del(context.Context, string) error { i.seen = false; return nil }

func TestRunFinishCleanupNotificationGateAndFailureOrder(t *testing.T) {
	for _, fault := range []string{"none", "template", "aggr", "lifecycle", "idem_read"} {
		t.Run(fault, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			factory, _ := finishRealLockerFactory(t)
			expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now()), SourceType: entity.SourceType_Workflow, ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 4}}
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).AnyTimes()
			quota := repomocks.NewMockQuotaRepo(ctrl)
			resources := 0
			quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), *entity.Session) error {
				resources++
				return nil
			}).AnyTimes()
			template := svcmocks.NewMockIExptTemplateManager(ctrl)
			aggr := svcmocks.NewMockExptAggrResultService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			pipeline := rpcmocks.NewMockIPipelineListAdapter(ctrl)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			calls := map[string]int{}
			shouldFail := func(step string) bool { calls[step]++; return fault == step && calls[step] == 1 }
			template.EXPECT().UpdateExptInfo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, entity.ExptStatus, int64, *int64) error {
				if shouldFail("template") {
					return errors.New("template fault")
				}
				return nil
			}).AnyTimes()
			aggr.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.AggrCalculateEvent, *time.Duration) error {
				if shouldFail("aggr") {
					return errors.New("aggr fault")
				}
				return nil
			}).AnyTimes()
			publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptLifecycleEvent, _ *time.Duration, key string) error {
				require.Equal(t, "expt_1_2_3_11", key)
				require.Equal(t, entity.ExptStatus_Processing, event.FromStatus)
				if shouldFail("lifecycle") {
					return errors.New("lifecycle fault")
				}
				return nil
			}).AnyTimes()
			pipeline.EXPECT().PipelineNodeFinishCallback(gomock.Any(), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64) error { calls["workflow"]++; return nil }).AnyTimes()
			metric.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(int64, int64, int64, time.Time) { calls["metric"]++ }).AnyTimes()
			sandbox := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
			expt.Target = &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}}
			sandbox.EXPECT().EmitExperimentFinished(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(metrics.SandboxAgentExperimentTags, error, time.Time, time.Time) { calls["sandbox_metric"]++ }).AnyTimes()
			idem := &cleanupTestIdem{}
			if fault == "idem_read" {
				idem.readErr = errors.New("idem unavailable")
			}
			config := confmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{}).AnyTimes()
			manager := &ExptMangerImpl{sandboxAgentMetrics: sandbox, configer: config, exptRepo: expts, quotaRepo: quota, manualRetryLocker: factory, idem: idem, templateManager: template, exptAggrResultService: aggr, publisher: publisher, pipelineListAdapter: pipeline, mtr: metric}
			event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
			err := manager.cleanupNaturallyFinishedRun(context.Background(), event, expt)
			if fault != "none" {
				require.Error(t, err)
				require.Zero(t, calls["workflow"], "failure before completed notification must not emit workflow")
				require.Zero(t, calls["metric"])
				require.Zero(t, calls["sandbox_metric"])
				idem.readErr = nil
				require.NoError(t, manager.cleanupNaturallyFinishedRun(context.Background(), event, expt))
			} else {
				require.NoError(t, err)
			}
			before := fmt.Sprint(calls)
			require.NoError(t, manager.cleanupNaturallyFinishedRun(context.Background(), event, expt))
			require.Equal(t, before, fmt.Sprint(calls), "normal duplicate repeats notifications")
			require.Equal(t, 1, calls["workflow"])
			require.Equal(t, 1, calls["metric"])
			require.Equal(t, 1, calls["sandbox_metric"])
			require.True(t, idem.seen)
			require.GreaterOrEqual(t, resources, 2, "resource retries must not be skipped by notification marker")
			require.Greater(t, idem.ttl, time.Hour)
		})
	}
}

func TestRunFinishCleanupSetFailureHasExplicitReplayWindow(t *testing.T) {
	ctrl := gomock.NewController(t)
	factory, _ := finishRealLockerFactory(t)
	expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).AnyTimes()
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(nil).Times(2)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	count := 0
	metric.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(int64, int64, int64, time.Time) { count++ }).Times(2)
	idem := &cleanupTestIdem{setErr: errors.New("marker unavailable")}
	manager := &ExptMangerImpl{exptRepo: expts, quotaRepo: quota, manualRetryLocker: factory, idem: idem, mtr: metric}
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	require.Error(t, manager.cleanupNaturallyFinishedRun(context.Background(), event, expt))
	require.False(t, idem.seen)
	require.Equal(t, 1, count)
	idem.setErr = nil
	require.NoError(t, manager.cleanupNaturallyFinishedRun(context.Background(), event, expt))
	require.True(t, idem.seen)
	require.Equal(t, 2, count, "marker failure after external success is a documented replay window")
}
func TestRunFinishCleanupMissingIdemFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	factory, _ := finishRealLockerFactory(t)
	expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil)
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(nil)
	manager := &ExptMangerImpl{exptRepo: expts, quotaRepo: quota, manualRetryLocker: factory, publisher: eventmocks.NewMockExptEventPublisher(ctrl), mtr: metricsmocks.NewMockExptMetric(ctrl)}
	require.True(t, errno.NeedMQRetry(manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, expt)))
}

func TestRunFinishCleanupMarkerDoesNotSkipResourceFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	factory, _ := finishRealLockerFactory(t)
	expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).Times(2)
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(errors.New("resource release unavailable"))
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(nil)
	manager := &ExptMangerImpl{exptRepo: expts, quotaRepo: quota, manualRetryLocker: factory, idem: &cleanupTestIdem{seen: true}, publisher: eventmocks.NewMockExptEventPublisher(ctrl), mtr: metricsmocks.NewMockExptMetric(ctrl)}
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
	require.Error(t, manager.cleanupNaturallyFinishedRun(context.Background(), event, expt))
	require.NoError(t, manager.cleanupNaturallyFinishedRun(context.Background(), event, expt))
}

func TestRunFinishRealCommitNotificationFailureRetry(t *testing.T) {
	for _, fault := range []string{"template", "aggr", "lifecycle"} {
		t.Run(fault, func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			p.publishSuccess(t, false)
			f := p.f
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"source_type": int32(entity.SourceType_Workflow), "expt_template_id": f.base + 700}).Error)
			template := svcmocks.NewMockIExptTemplateManager(ctrl)
			aggr := svcmocks.NewMockExptAggrResultService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			pipeline := rpcmocks.NewMockIPipelineListAdapter(ctrl)
			counts := map[string]int{}
			fail := func(step string) bool { counts[step]++; return fault == step && counts[step] == 1 }
			template.EXPECT().UpdateExptInfo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, entity.ExptStatus, int64, *int64) error {
				if fail("template") {
					return errors.New("template commit cleanup fault")
				}
				return nil
			}).AnyTimes()
			aggr.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.AggrCalculateEvent, *time.Duration) error {
				if fail("aggr") {
					return errors.New("aggr commit cleanup fault")
				}
				return nil
			}).AnyTimes()
			publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
				if fail("lifecycle") {
					return errors.New("lifecycle commit cleanup fault")
				}
				return nil
			}).AnyTimes()
			pipeline.EXPECT().PipelineNodeFinishCallback(gomock.Any(), f.expt, f.space).DoAndReturn(func(context.Context, int64, int64) error { counts["workflow"]++; return nil }).AnyTimes()
			p.manager.templateManager = template
			p.manager.exptAggrResultService = aggr
			p.manager.publisher = publisher
			p.manager.pipelineListAdapter = pipeline
			result, err := p.manager.FinishCurrentRun(ctx, p.event)
			require.True(t, errno.NeedMQRetry(err))
			require.Equal(t, entity.ExptRunFinishFinished, result.Disposition)
			require.Zero(t, counts["workflow"])
			require.Zero(t, p.notifications.Load())
			snapshot := f.snapshot()
			result, err = p.manager.FinishCurrentRun(ctx, p.event)
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishAlreadyFinished, result.Disposition)
			require.Equal(t, snapshot, f.snapshot())
			require.Equal(t, 1, counts["workflow"])
			require.Equal(t, int32(1), p.notifications.Load())
			calls := fmt.Sprint(counts)
			result, err = p.manager.FinishCurrentRun(ctx, p.event)
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishAlreadyFinished, result.Disposition)
			require.Equal(t, calls, fmt.Sprint(counts))
			require.Equal(t, int32(1), p.notifications.Load())
			require.Equal(t, int32(3), p.resources.Load())
			require.Equal(t, snapshot, f.snapshot())
		})
	}
}
