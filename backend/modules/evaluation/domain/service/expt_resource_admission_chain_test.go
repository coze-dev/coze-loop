// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
)

type reviewChainConfig struct{ resourceConfig }

func (reviewChainConfig) BuildEvalExt(context.Context, int64, *entity.Turn) map[string]string {
	return nil
}
func (reviewChainConfig) GetErrCtrl(context.Context) *entity.ExptErrCtrl {
	return &entity.ExptErrCtrl{}
}

type reviewChainMetric struct{ supplementMetric }

func (reviewChainMetric) EmitTurnExecEval(int64, int64)                                 {}
func (reviewChainMetric) EmitTurnExecResult(int64, int64, bool, bool, int64, time.Time) {}

type reviewChainLocker struct {
	lock.ILocker
	unlockedKey *string
}

func (l reviewChainLocker) Unlock(key string) (bool, error) {
	released, err := l.ILocker.Unlock(key)
	if released && err == nil {
		*l.unlockedKey = key
	}
	return released, err
}

func reviewChainExperiment(f *lateWriteFixture) *entity.Experiment {
	return &entity.Experiment{ID: f.expt, SpaceID: f.space, TargetID: 10, TargetVersionID: 20,
		TargetType: entity.EvalTargetTypeSandboxAgent, ExptDispatchMode: entity.ExptDispatchModeEnforce,
		Target: &entity.EvalTarget{ID: 10, EvalTargetType: entity.EvalTargetTypeSandboxAgent,
			EvalTargetVersion: &entity.EvalTargetVersion{ID: 20, EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{
			TargetVersionID: 20, IngressConf: &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}}}}}
}

func reviewChainItem(ctx context.Context, f *lateWriteFixture, event *entity.ExptItemEvalEvent) *entity.ExptItemEvalCtx {
	source := executionSource(ctx).Source
	item, err := f.itemRepo().GetItemRunLog(ctx, f.expt, f.run, f.item, f.space)
	require.NoError(f.t, err)
	return &entity.ExptItemEvalCtx{Event: event, Expt: reviewChainExperiment(f),
		EvalSetItem: &entity.EvaluationSetItem{ID: f.item, ItemVersionID: gptr.Of(f.version),
			BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1700000000))}, Turns: []*entity.Turn{{ID: 0}}},
		ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: item, TurnResultRunLogs: source.Turns}}
}

func TestResourceReviewD1OriginalItemAdmissionRetainsBudget(t *testing.T) {
	for _, entrance := range []string{"claimed_missing_invocation", "CallTarget_unknown_retry_identity"} {
		for _, budget := range []int{0, 3} {
			t.Run(fmt.Sprintf("%s/budget_%d_exhausted", entrance, budget), func(t *testing.T) {
				f, svc, guard, target, broker, event, ctx := reviewProtocolFixture(t)
				event.RetryTimes, event.MaxRetryTimes = budget, budget
				config := reviewChainConfig{resourceConfig{retry: budget}}
				svc.configer = config
				lockerFactory := svc.itemLocker
				unlockedKey := ""
				svc.itemLocker = func() lock.ILocker {
					return reviewChainLocker{ILocker: lockerFactory(), unlockedKey: &unlockedKey}
				}
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).
					Where("space_id=? AND item_id=?", f.space, f.item).Update("retry_times", budget).Error)
				item, err := f.itemRepo().GetItemRunLog(ctx, f.expt, f.run, f.item, f.space)
				require.NoError(t, err)
				claim, err := json.Marshal(entity.ResourceTurnStart{ItemRunLogID: item.ID, RequestID: event.RequestID, Claimed: true})
				require.NoError(t, err)
				ext, err := json.Marshal(map[string]string{"other": "keep", entity.ResourceTurnStartExtKey: string(claim)})
				require.NoError(t, err)
				targetID := int64(0)
				if entrance == "CallTarget_unknown_retry_identity" {
					targetID = target.record.ID
					target.record.Ext = nil
				}
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).
					Updates(map[string]any{"target_result_id": targetID, "evaluator_result_ids": nil, "ext": ext}).Error)
				executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), TurnResultRepo: f.turnRepo(),
					Configer: config, Metric: reviewChainMetric{}, evalTargetService: target,
					benefitService: supplementBenefit{}, evalAsyncRepo: supplementAsyncStore{}}
				var heldCtx context.Context
				svc.endpoints = RecordEvalChain(svc.HandleEventErr, svc.HandleEventLock, svc.HandleCurrentSource, svc.HandleCentralReservation)(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
					heldCtx = ctx
					return executor.Eval(ctx, reviewChainItem(ctx, f, event))
				})
				before := f.snapshot()
				err = svc.Eval(ctx, event)
				after := f.snapshot()
				t.Logf("REAL_MYSQL_ORIGINAL_ITEM_ADMISSION entrance=%s budget=%d broker_retry=%t unchanged=%t starts=%d lifecycle=%d",
					entrance, budget, errno.NeedMQRetry(err), reflect.DeepEqual(before, after), target.starts, len(guard.terminalItems))
				require.ErrorIs(t, err, errCentralItemAdmission)
				require.True(t, errno.NeedMQRetry(err))
				require.ErrorIs(t, heldCtx.Err(), context.Canceled, "item lease must unwind before broker retry")
				require.Equal(t, fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item), unlockedKey,
					"the actual item middleware must successfully unlock before returning broker retry")
				require.Equal(t, before, after, "neither turn nor item may Fail or consume retry authorization")
				require.Zero(t, target.starts)
				require.Empty(t, guard.terminalItems, "unknown physical attempt cannot complete/cancel the allocation")
				require.Zero(t, guard.legacyReleases)
				require.Nil(t, broker.event, "admission must not publish a business execution retry")
			})
		}
	}
}

func TestResourceReviewC2TurnExtPreservationDoesNotChangeLegacy(t *testing.T) {
	for _, payload := range []string{"", "opaque-legacy-extension"} {
		t.Run(payload, func(t *testing.T) {
			f, _, _, _, _, event, _ := reviewProtocolFixture(t)
			source := f.source()
			ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source, SourceValidated: true})
			item := reviewChainItem(ctx, f, event)
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), Configer: reviewChainConfig{}}
			etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: item, Turn: &entity.Turn{ID: 0}, ExptTurnRunResult: &entity.ExptTurnRunResult{}}
			if payload != "" {
				etec.Ext = map[string]string{entity.ResourceTurnStartExtKey: payload}
			}
			require.NoError(t, executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{}))
			if payload == "" {
				require.Nil(t, f.source().Turns[0].Ext, "legacy nil Ext must remain nil")
			} else {
				require.Equal(t, payload, f.source().Turns[0].Ext[entity.ResourceTurnStartExtKey], "non-resource Ext remains opaque")
			}
		})
	}
}

func TestResourceReviewD1OrdinaryExecutionAndLegacyCompletionStayTerminal(t *testing.T) {
	for _, kind := range []string{"resource_business_error", "legacy_error"} {
		t.Run(kind, func(t *testing.T) {
			f, svc, guard, _, _, event, ctx := reviewProtocolFixture(t)
			event.MaxRetryTimes = 0
			config := reviewChainConfig{}
			svc.configer = config
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), Configer: config}
			executionErr := errors.New("known terminal execution failure")
			if kind == "legacy_error" {
				// The new broker classification is resource-only; preserve the old non-resource boundary.
				executionErr = errCentralItemAdmission
				ctx = context.WithValue(ctx, centralResourceExecutionKey{}, false)
				source := f.source()
				ctx = context.WithValue(ctx, currentItemSourceKey{}, &currentItemExecution{Source: source, SourceValidated: true})
				require.NoError(t, executor.CompleteItemRun(ctx, reviewChainItem(ctx, f, event), executionErr))
			} else {
				svc.endpoints = svc.HandleEventErr(svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
					return executor.CompleteItemRun(ctx, reviewChainItem(ctx, f, event), executionErr)
				}))
				require.NoError(t, svc.Eval(ctx, event))
				require.Len(t, guard.terminalItems, 1)
			}
			require.Equal(t, entity.ItemRunState_Fail, f.source().Current.Status)
			require.Zero(t, guard.legacyReleases)
		})
	}
}

func TestResourceReviewC2ResultCannotClearResourceClaim(t *testing.T) {
	for _, poison := range []bool{false, true} {
		t.Run(fmt.Sprint(poison), func(t *testing.T) {
			f, _, _, _, _, event, _ := reviewProtocolFixture(t)
			item, err := f.itemRepo().GetItemRunLog(context.Background(), f.expt, f.run, f.item, f.space)
			require.NoError(t, err)
			execution := &currentItemExecution{Source: f.source(), SourceValidated: true, ItemRunLogID: item.ID}
			claim := resourceTurnStartValue(execution, true)
			ext, err := json.Marshal(map[string]string{entity.ResourceTurnStartExtKey: claim})
			require.NoError(t, err)
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("ext", ext).Error)
			execution.Source = f.source()
			ctx := context.WithValue(context.Background(), currentItemSourceKey{}, execution)
			ctx = context.WithValue(ctx, centralResourceExecutionKey{}, true)
			etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: reviewChainItem(ctx, f, event),
				Turn: &entity.Turn{ID: 0}, ExptTurnRunResult: &entity.ExptTurnRunResult{}}
			if poison {
				etec.Ext = map[string]string{entity.ResourceTurnStartExtKey: resourceTurnStartValue(execution, false)}
			}
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), Configer: reviewChainConfig{}}
			require.NoError(t, executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{}))
			require.Equal(t, claim, f.source().Turns[0].Ext[entity.ResourceTurnStartExtKey])
		})
	}
}
