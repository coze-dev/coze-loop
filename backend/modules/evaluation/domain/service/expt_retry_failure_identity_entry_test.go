// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

// Keep the real result reader in this chain: its legacy map conversion can lose instance identities.
func TestRetryFailure_IdentityThroughResultEntry(t *testing.T) {
	for _, tc := range []struct {
		name               string
		readError          string
		alias, sameVersion bool
		inline, noTarget   bool
	}{
		{name: "legacy_success_control"},
		{name: "single_alias_success", alias: true},
		{name: "same_version_alias_success_and_failure", alias: true, sameVersion: true},
		{name: "no_target_single_alias_success", alias: true, noTarget: true},
		{name: "no_target_same_version_aliases", alias: true, sameVersion: true, noTarget: true},
		{name: "inline_success_and_builtin_failure", inline: true},
		{name: "no_target_inline_success", inline: true, noTarget: true},
		{name: "target_read_failure_preserves_source", readError: "target"},
		{name: "evaluator_read_failure_preserves_source", readError: "evaluator"},
		{name: "source_runlog_read_failure_preserves_source", readError: "source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := context.Background()
			const exptID, oldRunID, runID, spaceID, itemID, turnID, turnResultID = int64(1), int64(101), int64(103), int64(3), int64(10), int64(20), int64(21)
			successVersion, failedVersion := int64(41), int64(42)
			successAlias, failedAlias := "", ""
			if tc.alias {
				successAlias, failedAlias = "judge-a", "judge-b"
			}
			if tc.sameVersion {
				failedVersion = successVersion
			}
			targetID := int64(201)
			if tc.noTarget {
				targetID = 0
			}
			success := &entity.EvaluatorRecord{ID: 301, SpaceID: spaceID, ExperimentID: exptID, ExperimentRunID: oldRunID, ItemID: itemID, TurnID: turnID, EvaluatorVersionID: successVersion, Alias: successAlias, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess}
			failed := &entity.EvaluatorRecord{ID: 302, SpaceID: spaceID, ExperimentID: exptID, ExperimentRunID: oldRunID, ItemID: itemID, TurnID: turnID, EvaluatorVersionID: failedVersion, Alias: failedAlias, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusFail}
			if tc.inline {
				success.EvaluatorVersionID, success.InlineKey, success.SourceType = 0, "inline-quality", entity.EvaluatorRecordSourceTypeInline
			}
			refs := []*entity.ExptTurnEvaluatorResultRef{
				{ID: 31, SpaceID: spaceID, ExptID: exptID, ExptTurnResultID: turnResultID, EvaluatorVersionID: success.EvaluatorVersionID, EvaluatorResultID: success.ID, Alias: success.Alias, InlineKey: success.InlineKey, SourceType: int32(success.SourceType)},
				{ID: 32, SpaceID: spaceID, ExptID: exptID, ExptTurnResultID: turnResultID, EvaluatorVersionID: failedVersion, EvaluatorResultID: failed.ID, Alias: failedAlias, SourceType: int32(failed.SourceType)},
			}
			sourceResults := &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: failedVersion, Alias: failedAlias, RecordID: failed.ID}}}
			if tc.inline {
				sourceResults.Inline = []*entity.InlineEvalResult{{InlineKey: success.InlineKey, RecordID: success.ID}}
			} else {
				sourceResults.Registered = append(sourceResults.Registered, &entity.RegisteredEvalResult{VersionID: successVersion, Alias: successAlias, RecordID: success.ID})
			}
			expt := &entity.Experiment{
				ID: exptID, SpaceID: spaceID, TargetVersionID: 1, TargetType: 1, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
				EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(1)}}},
			}
			if tc.noTarget {
				expt.TargetVersionID = 0
			}
			itemConfig := &entity.ExptItemConfig{}
			versions := make(map[int64]bool)
			for _, record := range []*entity.EvaluatorRecord{success, failed} {
				if record.SourceType == entity.EvaluatorRecordSourceTypeInline {
					continue
				}
				itemConfig.EvaluatorConfs = append(itemConfig.EvaluatorConfs, &entity.ItemEvaluatorConf{EvaluatorVersionID: record.EvaluatorVersionID, Alias: record.Alias})
				if versions[record.EvaluatorVersionID] {
					continue
				}
				versions[record.EvaluatorVersionID] = true
				expt.Evaluators = append(expt.Evaluators, &entity.Evaluator{ID: record.EvaluatorVersionID, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: record.EvaluatorVersionID}})
				expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf = append(expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf, &entity.EvaluatorConf{EvaluatorVersionID: record.EvaluatorVersionID, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}, TargetAdapter: &entity.FieldAdapter{}}})
			}
			turn := &entity.Turn{ID: turnID, ItemID: itemID}
			eiec := &entity.ExptItemEvalCtx{
				Expt: expt, ItemConfig: itemConfig,
				Event:               &entity.ExptItemEvalEvent{ExptID: exptID, ExptRunID: runID, SpaceID: spaceID, EvalSetItemID: itemID, ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"}},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: itemID, Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: make(map[int64]*entity.ExptTurnResultRunLog)},
			}
			turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
			itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
			idgen := idgenmocks.NewMockIIDGenerator(ctrl)
			targetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
			recordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
			target := &entity.EvalTargetRecord{ID: targetID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}}
			turnRepo.EXPECT().GetItemTurnResults(gomock.Any(), exptID, itemID, spaceID).DoAndReturn(func(ctx context.Context, _, _, _ int64) ([]*entity.ExptTurnResult, error) {
				assert.True(t, contexts.CtxWriteDB(ctx))
				return []*entity.ExptTurnResult{{ID: turnResultID, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, TurnID: turnID, TargetResultID: targetID}}, nil
			})
			turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), spaceID, []int64{turnResultID}).DoAndReturn(func(ctx context.Context, _ int64, _ []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
				assert.True(t, contexts.CtxWriteDB(ctx))
				return refs, nil
			}).AnyTimes()
			dependencyErr := errors.New("temporary reuse dependency failure")
			var targetErr, sourceErr error
			if tc.readError == "target" {
				targetErr = dependencyErr
			}
			if tc.readError == "source" {
				sourceErr = dependencyErr
			}
			turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), exptID, oldRunID, []int64{itemID}, spaceID).Return([]*entity.ExptTurnResultRunLog{{SpaceID: spaceID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, TurnID: turnID, TargetResultID: targetID, EvaluatorResultIds: sourceResults}}, sourceErr).AnyTimes()
			targetSvc.EXPECT().GetRecordByID(gomock.Any(), spaceID, targetID).Return(target, targetErr).AnyTimes()
			recordSvc.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).DoAndReturn(func(ctx context.Context, ids []int64, _, _ bool) ([]*entity.EvaluatorRecord, error) {
				assert.True(t, contexts.CtxWriteDB(ctx))
				if tc.readError == "evaluator" {
					return nil, dependencyErr
				}
				var result []*entity.EvaluatorRecord
				for _, id := range ids {
					if id == success.ID {
						result = append(result, success)
					}
					if id == failed.ID {
						result = append(result, failed)
					}
				}
				return result, nil
			}).AnyTimes()
			idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{901}, nil).AnyTimes()
			var persisted []*entity.ExptTurnResultRunLog
			turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error { persisted = rows; return nil }).AnyTimes()
			reader := &ExptResultServiceImpl{ExptTurnResultRepo: turnRepo}
			mode := &ExptRecordEvalModeFailRetry{resultSvc: reader, exptTurnResultRepo: turnRepo, idgen: idgen, evalTargetService: targetSvc, evaluatorRecordSvc: recordSvc}
			preErr := mode.PreEval(ctx, eiec)
			if tc.readError != "" {
				assert.ErrorIs(t, preErr, dependencyErr, "an unavailable dependency must stop the real PreEval entry")
				assert.Empty(t, persisted, "do not persist a reset snapshot after an inconclusive dependency read")
				assert.Empty(t, eiec.ExistItemEvalResult.TurnResultRunLogs)
				assert.Equal(t, entity.EvaluatorRunStatusSuccess, success.Status)
				assert.Equal(t, int64(301), refs[0].EvaluatorResultID)
				return
			}
			require.NoError(t, preErr)
			require.Len(t, persisted, 1)
			assert.ElementsMatch(t, []int64{success.ID}, retryIdentityEntryIDs(persisted[0].EvaluatorResultIds), "the real result reader must preserve the successful instance identity")
			config := configmocks.NewMockIConfiger(ctrl)
			config.EXPECT().BuildEvalExt(gomock.Any(), spaceID, turn).Return(nil)
			itemRepo.EXPECT().BatchGet(gomock.Any(), spaceID, exptID, []int64{itemID}).Return(nil, nil)
			executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, ItemResultRepo: itemRepo, Configer: config, evalTargetService: targetSvc, evaluatorRecordService: recordSvc}
			etec, err := executor.buildExptTurnEvalCtx(ctx, turn, eiec, nil)
			require.NoError(t, err)
			var loaded []int64
			for _, record := range etec.ExptTurnRunResult.EvaluatorResults {
				loaded = append(loaded, record.ID)
			}
			assert.ElementsMatch(t, []int64{success.ID}, loaded, "execution context must load the exact retained record")
			evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
			benefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			benefitSvc.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil).AnyTimes()
			evaluatorSvc.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
			var mu sync.Mutex
			var invoked []string
			evaluatorSvc.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				mu.Lock()
				defer mu.Unlock()
				invoked = append(invoked, fmt.Sprintf("%d:%s", req.EvaluatorVersionID, req.Alias))
				return &entity.EvaluatorRecord{ID: 1000 + int64(len(invoked)), EvaluatorVersionID: req.EvaluatorVersionID, Alias: req.Alias, Status: entity.EvaluatorRunStatusSuccess}, nil
			}).AnyTimes()
			metric.EXPECT().EmitTurnExecEvaluatorResult(spaceID, false).AnyTimes()
			turnEval := &DefaultExptTurnEvaluationImpl{metric: metric, evalTargetService: targetSvc, evaluatorService: evaluatorSvc, benefitService: benefitSvc, evaluatorRecordService: recordSvc}
			actualTarget, err := turnEval.CallTarget(ctx, etec)
			require.NoError(t, err)
			assert.Equal(t, targetID, actualTarget.ID)
			results, err := turnEval.CallEvaluators(ctx, etec, actualTarget)
			require.NoError(t, err)
			assert.Equal(t, []string{fmt.Sprintf("%d:%s", failedVersion, failedAlias)}, invoked, "only the failed evaluator may be invoked")
			if !tc.inline {
				var retained *entity.EvaluatorRecord
				for _, record := range results {
					if record.ID == success.ID {
						retained = record
					}
				}
				assert.Same(t, success, retained)
			}
		})
	}
}

func retryIdentityEntryIDs(results *entity.EvaluatorResults) []int64 {
	if results == nil {
		return nil
	}
	var ids []int64
	for _, ref := range results.Registered {
		if ref != nil {
			ids = append(ids, ref.RecordID)
		}
	}
	for _, ref := range results.Inline {
		if ref != nil {
			ids = append(ids, ref.RecordID)
		}
	}
	for _, id := range results.EvalVerIDToResID {
		ids = append(ids, id)
	}
	return ids
}
