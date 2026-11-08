// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

// Exact e6 event fields: e6f46429:domain/entity/event.go. The round trip below
// actually loses RequestID/ResumeSource before the final consumer reads it.
type normalE6Event struct {
	SpaceID                     int64
	ExptID                      int64
	ExptRunID                   int64
	ExptRunMode                 entity.ExptRunMode
	EvalSetItemID               int64
	AsyncReportTrigger          bool
	AsyncEvaluatorReportTrigger bool
	CreateAt                    int64
	RetryTimes                  int
	MaxRetryTimes               int
	Ext                         map[string]string
	Session                     *entity.Session
}

func TestNormalAsyncMissingRequestBindsRealCurrentReference(t *testing.T) {
	for _, kind := range []string{"record_target", "bare_target", "record_evaluator", "bare_evaluator", "old_explicit_request", "old_record", "ref_missing", "invoking"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
			manager := svcmocks.NewMockIExptManager(ctrl)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: 9, ExptID: 7, RunID: 8, ItemID: 10, ItemVersionID: 11, RequestID: "current-request", Mode: entity.EvaluationModeRetryItems, Status: entity.ItemRunState_Processing}, Turns: map[int64]*entity.ExptTurnResultRunLog{0: {ID: 21, SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 10, ItemVersionID: 11, TurnID: 0, Status: entity.TurnRunState_Processing, TargetResultID: 71}}}
			original := &entity.ExptItemEvalEvent{SpaceID: 9, ExptID: 7, ExptRunID: 8, EvalSetItemID: 10, ExptRunMode: entity.EvaluationModeSubmit, RequestID: "current-request", AsyncReportTrigger: true, MaxRetryTimes: 2}
			raw, err := json.Marshal(original)
			require.NoError(t, err)
			old := &normalE6Event{}
			require.NoError(t, json.Unmarshal(raw, old))
			raw, err = json.Marshal(old)
			require.NoError(t, err)
			event := &entity.ExptItemEvalEvent{}
			require.NoError(t, json.Unmarshal(raw, event))
			require.Empty(t, event.RequestID)
			evaluator := kind == "record_evaluator" || kind == "bare_evaluator"
			if evaluator {
				event.AsyncReportTrigger = false
				event.AsyncEvaluatorReportTrigger = true
				source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 81}}}
			}
			if kind == "record_target" || kind == "old_record" || kind == "invoking" || kind == "ref_missing" {
				event = event.CloneWithResumeSource("target", 71, 0)
			}
			if kind == "record_evaluator" {
				event = event.CloneWithResumeSource("evaluator", 81, 0)
			}
			if kind == "old_explicit_request" {
				event.RequestID = "old-request"
			}
			if kind == "old_record" {
				event.ResumeSource.RecordID = 72
				event.ResumeSource.InvokeID = "72"
			}
			if kind == "ref_missing" {
				source.Turns[0].TargetResultID = 0
			}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(&source.Current, nil)
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(source, nil).AnyTimes()
			expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(&entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing}, nil).AnyTimes()
			manager.EXPECT().GetRunLog(gomock.Any(), int64(7), int64(8), int64(9), gomock.Any()).Return(&entity.ExptRunLog{SpaceID: 9, ExptID: 7, ExptRunID: 8, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
			manager.EXPECT().GetDetail(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(&entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, TargetVersionID: 31}, nil).AnyTimes()
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			status := entity.EvalTargetRunStatusSuccess
			if kind == "invoking" {
				status = entity.EvalTargetRunStatusAsyncInvoking
			}
			target.EXPECT().GetRecordByID(gomock.Any(), int64(9), int64(71)).Return(&entity.EvalTargetRecord{ID: 71, SpaceID: 9, ExperimentRunID: 8, ItemID: 10, ItemVersionID: 11, TurnID: 0, TargetVersionID: 31, Status: gptr.Of(status)}, nil).AnyTimes()
			records := svcmocks.NewMockEvaluatorRecordService(ctrl)
			records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(81), false).Return(&entity.EvaluatorRecord{ID: 81, SpaceID: 9, ExperimentID: 7, ExperimentRunID: 8, ItemID: 10, ItemVersionID: 11, TurnID: 0, EvaluatorVersionID: 51, Alias: "a", TargetRecordID: 71, Status: entity.EvaluatorRunStatusSuccess}, nil).AnyTimes()
			port.EXPECT().CompleteCurrentItemRun(gomock.Any(), source, gomock.Any()).Return(true, nil).AnyTimes()
			factory, _ := finishRealLockerFactory(t)
			svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: &legacyWakeSourceRepo{IExptItemResultRepo: items, IExptCurrentResultRepo: port}, exptTurnResultRepo: repomocks.NewMockIExptTurnResultRepo(ctrl), manager: manager, experimentRepo: expts, itemLocker: factory, evaTargetService: target, evaluatorRecordService: records}
			called := false
			err = svc.HandleEventLock(svc.HandleCurrentSource(func(_ context.Context, e *entity.ExptItemEvalEvent) error {
				called = true
				require.Equal(t, "current-request", e.RequestID)
				require.Equal(t, entity.EvaluationModeRetryItems, e.ExptRunMode)
				require.NotNil(t, e.ResumeSource)
				return nil
			}))(context.Background(), event)
			wants := kind == "record_target" || kind == "bare_target" || kind == "record_evaluator" || kind == "bare_evaluator"
			require.Equal(t, wants, called, "BUSINESS_RED valid persisted current callback must continue after old codec loses Q")
			if kind == "ref_missing" || kind == "invoking" {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
