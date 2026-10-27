// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func publishUnitTurn(turnID int64) *entity.ExptTurnResult {
	return &entity.ExptTurnResult{ID: 100, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: turnID}
}

func publishUnitLog(turnID int64) *entity.ExptTurnResultRunLog {
	return &entity.ExptTurnResultRunLog{ID: 101, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, TurnID: turnID, Status: entity.TurnRunState_Success}
}

func publishUnitSource() *entity.ExptItemResultPublishSource {
	turn := publishUnitTurn(0)
	log := publishUnitLog(0)
	source := &entity.ExptCurrentItemRunSource{
		Current: entity.ExptItemCurrentRequest{
			SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 9, ItemVersionID: 7,
			RequestID: "req", Mode: entity.EvaluationModeFailRetry, Status: entity.ItemRunState_Success,
		},
		CanonicalTurns: []*entity.ExptTurnResult{turn},
		Turns:          map[int64]*entity.ExptTurnResultRunLog{0: log},
	}
	return &entity.ExptItemResultPublishSource{
		Source:  source,
		Item:    &entity.ExptItemResult{ID: 1, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 9, ItemVersionID: 7, Status: entity.ItemRunState_Success},
		ItemLog: &entity.ExptItemResultRunLog{ItemID: 9, Status: int32(entity.ItemRunState_Success), ResultState: int32(entity.ExptItemResultStateLogged)},
	}
}

func publishUnitExperiment() *entity.Experiment {
	return &entity.Experiment{ID: 1, SpaceID: 3}
}

type publishUnitDeps struct {
	ctrl    *gomock.Controller
	repo    *repomocks.MockIExptResultPublishRepo
	locker  *lockmocks.MockILocker
	targets *svcmocks.MockIEvalTargetService
	records *svcmocks.MockEvaluatorRecordService
	idgen   *idmocks.MockIIDGenerator
	svc     *ExptResultServiceImpl
}

func newPublishUnitDeps(t *testing.T) *publishUnitDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := &publishUnitDeps{
		ctrl:    ctrl,
		repo:    repomocks.NewMockIExptResultPublishRepo(ctrl),
		locker:  lockmocks.NewMockILocker(ctrl),
		targets: svcmocks.NewMockIEvalTargetService(ctrl),
		records: svcmocks.NewMockEvaluatorRecordService(ctrl),
		idgen:   idmocks.NewMockIIDGenerator(ctrl),
	}
	d.svc = &ExptResultServiceImpl{
		resultPublishRepo:      d.repo,
		resultPublishLocker:    func() lock.ILocker { return d.locker },
		evalTargetService:      d.targets,
		evaluatorRecordService: d.records,
		idgen:                  d.idgen,
	}
	return d
}

func TestPublicationReadyUnit(t *testing.T) {
	require.False(t, publicationReady(nil))
	require.False(t, publicationReady(&entity.ExptItemResultPublishSource{}))
	ready := publishUnitSource()
	require.True(t, publicationReady(ready))

	notLogged := publishUnitSource()
	notLogged.ItemLog.ResultState = int32(entity.ExptItemResultStateResulted)
	require.False(t, publicationReady(notLogged))

	unfinished := publishUnitSource()
	unfinished.ItemLog.Status = int32(entity.ItemRunState_Processing)
	require.False(t, publicationReady(unfinished))
}

func TestGetItemResultPublishSourceUnit(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		svc := &ExptResultServiceImpl{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := svc.GetItemResultPublishSource(ctx, 3, 1, 2, 9)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("repo_missing", func(t *testing.T) {
		svc := &ExptResultServiceImpl{}
		_, err := svc.GetItemResultPublishSource(context.Background(), 3, 1, 2, 9)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("repo_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.repo.EXPECT().GetItemResultPublishSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read"))
		_, err := d.svc.GetItemResultPublishSource(context.Background(), 3, 1, 2, 9)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("success", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		d.repo.EXPECT().GetItemResultPublishSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(source, nil)
		got, err := d.svc.GetItemResultPublishSource(context.Background(), 3, 1, 2, 9)
		require.NoError(t, err)
		require.Same(t, source, got)
	})
}

func TestPrepareItemResultPublicationUnit(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := d.svc.PrepareItemResultPublication(ctx, publishUnitSource(), publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("not_ready", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		publication, err := d.svc.PrepareItemResultPublication(context.Background(), &entity.ExptItemResultPublishSource{}, publishUnitExperiment())
		require.NoError(t, err)
		require.Nil(t, publication)
	})
	t.Run("source_incomplete", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Item = nil
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))

		source = publishUnitSource()
		source.Source.CanonicalTurns = nil
		_, err = d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("experiment_mismatch", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), publishUnitSource(), nil)
		require.True(t, errno.NeedMQRetry(err))

		_, err = d.svc.PrepareItemResultPublication(context.Background(), publishUnitSource(), &entity.Experiment{ID: 9, SpaceID: 3})
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("canonical_turn_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.CanonicalTurns = []*entity.ExptTurnResult{nil}
		source.Source.Turns = map[int64]*entity.ExptTurnResultRunLog{0: publishUnitLog(0)}
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("frozen_turn_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns = map[int64]*entity.ExptTurnResultRunLog{}
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("multiset_config_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].TargetResultID = 88
		expt := publishUnitExperiment()
		expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, expt)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_reader_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.svc.evalTargetService = nil
		source := publishUnitSource()
		source.Source.Turns[0].TargetResultID = 88
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_read_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].TargetResultID = 88
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(88)).Return(nil, errors.New("read"))
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_foreign", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].TargetResultID = 88
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(88)).Return(&entity.EvalTargetRecord{ID: 88, SpaceID: 3, ItemID: 99, TurnID: 0, ExperimentRunID: 2, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("target_record_valid", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].TargetResultID = 88
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(88)).Return(&entity.EvalTargetRecord{ID: 88, SpaceID: 3, ItemID: 9, TurnID: 0, ExperimentRunID: 2, ItemVersionID: 7, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
		publication, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.NoError(t, err)
		require.Len(t, publication.Turns, 1)
		require.Equal(t, int64(88), publication.Turns[0].TargetResultID)
	})
	t.Run("registered_ref_valid", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{61}, false, false).Return([]*entity.EvaluatorRecord{{
			ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			EvaluatorVersionID: 51, Alias: "a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
			EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.5)}},
		}}, nil)
		d.idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{900}, nil)
		publication, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.NoError(t, err)
		require.Len(t, publication.Refs, 1)
		require.Equal(t, int64(900), publication.Refs[0].ID)
		require.NotNil(t, publication.Turns[0].WeightedScore)
	})
	t.Run("evaluator_reader_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.svc.evaluatorRecordService = nil
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_read_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("read"))
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_record_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_record_duplicated", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		record := &entity.EvaluatorRecord{ID: 61}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{record, record}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("inline_source_mismatch", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "k", RecordID: 62}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{62}, false, false).Return([]*entity.EvaluatorRecord{{
			ID: 62, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "other", Status: entity.EvaluatorRunStatusSuccess,
		}}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("registered_source_mismatch", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{
			ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			EvaluatorVersionID: 52, Alias: "a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
		}}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("existing_ref_reused", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		source.Source.CurrentRefs = []*entity.ExptTurnEvaluatorResultRef{{ID: 700, ExptTurnResultID: 100, EvaluatorVersionID: 51, EvaluatorResultID: 61, Alias: "a", SourceType: int32(entity.EvaluatorRecordSourceTypeBuiltin)}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{
			ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			EvaluatorVersionID: 51, Alias: "a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
		}}, nil)
		publication, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.NoError(t, err)
		require.Len(t, publication.Refs, 1)
		require.Equal(t, int64(700), publication.Refs[0].ID)
	})
	t.Run("idgen_missing", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.svc.idgen = nil
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{
			ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			EvaluatorVersionID: 51, Alias: "a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
		}}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("idgen_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{
			ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			EvaluatorVersionID: 51, Alias: "a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
		}}, nil)
		d.idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return(nil, errors.New("idgen"))
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("idgen_incomplete", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{
			ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0,
			EvaluatorVersionID: 51, Alias: "a", SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess,
		}}, nil)
		d.idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, publishUnitExperiment())
		require.True(t, errno.NeedMQRetry(err))
	})
}

func TestPublishPreparedItemResultUnit(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := d.svc.PublishPreparedItemResult(ctx, &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("nil_publication", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		applied, err := d.svc.PublishPreparedItemResult(context.Background(), nil, nil)
		require.NoError(t, err)
		require.False(t, applied)
	})
	t.Run("invalid_source", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: &entity.ExptItemResultPublishSource{}}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("deps_missing", func(t *testing.T) {
		svc := &ExptResultServiceImpl{}
		_, err := svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("locker_nil", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.svc.resultPublishLocker = func() lock.ILocker { return nil }
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("lock_busy", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("source_check_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).Return(false, errors.New("check"))
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("not_current", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(false, errors.New("unlock"))
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).Return(false, nil)
		applied, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.NoError(t, err)
		require.False(t, applied)
	})
	t.Run("before_commit_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).Return(true, nil)
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, func(context.Context, *entity.ExptItemResultPublishSource) error {
			return errors.New("before")
		})
		require.Error(t, err)
	})
	t.Run("commit_error", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).Return(true, nil)
		d.repo.EXPECT().CommitItemResult(gomock.Any(), gomock.Any()).Return(false, errors.New("commit"))
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("success", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).Return(true, nil)
		d.repo.EXPECT().CommitItemResult(gomock.Any(), gomock.Any()).Return(true, nil)
		applied, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, func(context.Context, *entity.ExptItemResultPublishSource) error { return nil })
		require.NoError(t, err)
		require.True(t, applied)
	})
}

func TestRecordCurrentItemResultsUnit(t *testing.T) {
	event := func() *entity.ExptScheduleEvent {
		return &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}
	}
	expt := publishUnitExperiment()
	readyItem := func() *entity.ExptEvalItem {
		source := publishUnitSource()
		return &entity.ExptEvalItem{ItemID: 9, RequestID: "req", Publication: source, State: entity.ItemRunState_Success}
	}
	newDeps := func(t *testing.T) (*publishUnitDeps, *entitymocks.MockExptSchedulerMode, *ExptSchedulerImpl) {
		d := newPublishUnitDeps(t)
		mode := entitymocks.NewMockExptSchedulerMode(d.ctrl)
		scheduler := &ExptSchedulerImpl{}
		return d, mode, scheduler
	}

	t.Run("cancelled", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, errno.NeedMQRetry(scheduler.recordCurrentItemResults(ctx, event(), nil, mode, expt, nil)))
	})
	t.Run("nil_item", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		require.True(t, errno.NeedMQRetry(scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{nil}, mode, expt, nil)))
	})
	t.Run("observed_without_source", func(t *testing.T) {
		d, mode, scheduler := newDeps(t)
		_ = d
		item := &entity.ExptEvalItem{ItemID: 9, PublicationObserved: true}
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, nil))
	})
	t.Run("legacy_source_error", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(gomock.NewController(t))
		svc.EXPECT().GetItemResultPublishSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read"))
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Success}
		require.Error(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("legacy_source_mismatch", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(gomock.NewController(t))
		source := publishUnitSource()
		source.Source.Current.RequestID = "other"
		svc.EXPECT().GetItemResultPublishSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Success}
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("not_ready", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(gomock.NewController(t))
		source := publishUnitSource()
		source.ItemLog.Status = int32(entity.ItemRunState_Processing)
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Processing, Publication: source}
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("scope_mismatch", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(gomock.NewController(t))
		source := publishUnitSource()
		source.Source.Current.SpaceID = 99
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Success, Publication: source}
		require.True(t, errno.NeedMQRetry(scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc)))
	})
	t.Run("prepare_error", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(gomock.NewController(t))
		item := readyItem()
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), item.Publication, expt).Return(nil, errors.New("prepare"))
		require.Error(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("not_applied", func(t *testing.T) {
		_, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(gomock.NewController(t))
		item := readyItem()
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), item.Publication, expt).Return(&entity.ExptItemResultPublication{Source: item.Publication}, nil)
		svc.EXPECT().PublishPreparedItemResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("applied_publish_result_error_is_logged", func(t *testing.T) {
		d, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(d.ctrl)
		results := svcmocks.NewMockExptResultService(d.ctrl)
		publisher := eventmocks.NewMockExptEventPublisher(d.ctrl)
		scheduler.ResultSvc = results
		scheduler.Publisher = publisher
		item := readyItem()
		publication := &entity.ExptItemResultPublication{Source: item.Publication}
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), item.Publication, expt).Return(publication, nil)
		svc.EXPECT().PublishPreparedItemResult(gomock.Any(), publication, gomock.Any()).Return(true, nil)
		mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish"))
		results.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).Return(errors.New("upsert"))
		publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("applied_publisher_error", func(t *testing.T) {
		d, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(d.ctrl)
		publisher := eventmocks.NewMockExptEventPublisher(d.ctrl)
		results := svcmocks.NewMockExptResultService(d.ctrl)
		scheduler.ResultSvc = results
		scheduler.Publisher = publisher
		item := readyItem()
		publication := &entity.ExptItemResultPublication{Source: item.Publication}
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), item.Publication, expt).Return(publication, nil)
		svc.EXPECT().PublishPreparedItemResult(gomock.Any(), publication, gomock.Any()).Return(true, nil)
		mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		results.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).Return(nil)
		publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish"))
		require.Error(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
	t.Run("applied_without_publisher", func(t *testing.T) {
		d, mode, scheduler := newDeps(t)
		svc := svcmocks.NewMockExptResultPublicationService(d.ctrl)
		results := svcmocks.NewMockExptResultService(d.ctrl)
		scheduler.ResultSvc = results
		item := readyItem()
		publication := &entity.ExptItemResultPublication{Source: item.Publication}
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), item.Publication, expt).Return(publication, nil)
		svc.EXPECT().PublishPreparedItemResult(gomock.Any(), publication, gomock.Any()).Return(true, nil)
		mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		results.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{9}).Return(nil)
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, mode, expt, svc))
	})
}

var _ = time.Now

type filterManualRepo struct {
	*repomocks.MockIExptItemResultRepo
	*repomocks.MockIExptManualRetryFilterRepo
}

type materializedFilterRepo struct {
	*repomocks.MockIExptTurnResultFilterRepo
	*repomocks.MockIExptMaterializedFilterRepo
}

func (m *materializedFilterRepo) PersistsMaterializedItemVersion() bool { return true }

func TestFilterPureHelpersUnit(t *testing.T) {
	require.Equal(t, []int64{1, 2, 3}, sortedFilterItems([]int64{3, 1, 2, 1}))
	zero := 0.0
	require.True(t, filterWeightedScoreEqual(nil, nil))
	require.True(t, filterWeightedScoreEqual(&zero, nil))
	require.False(t, filterWeightedScoreEqual(nil, &zero))
	one := 1.0
	require.True(t, filterWeightedScoreEqual(&one, &one))
}

func TestWithFilterItemLockUnit(t *testing.T) {
	ctx := context.Background()
	t.Run("cancelled", func(t *testing.T) {
		svc := &ExptResultServiceImpl{}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(cancelled, 1, 9, func(context.Context) error { return nil })))
	})
	t.Run("current_repo_without_locker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := &ExptResultServiceImpl{ExptItemResultRepo: &currentUnitRepo{MockIExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl), MockIExptCurrentResultRepo: repomocks.NewMockIExptCurrentResultRepo(ctrl)}}
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return nil })))
	})
	t.Run("legacy_repo_runs_without_lock", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := &ExptResultServiceImpl{ExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl)}
		called := false
		require.NoError(t, svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { called = true; return nil }))
		require.True(t, called)
	})
	t.Run("nil_locker", func(t *testing.T) {
		svc := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return nil }}
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return nil })))
	})
	t.Run("lock_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:1:9", gomock.Any(), gomock.Any()).Return(false, nil, nil, errors.New("lock"))
		svc := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return nil })))
	})
	t.Run("busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, ctx, func() {}, nil)
		svc := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return nil })))
	})
	t.Run("lock_ctx_cancelled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, cancelled, func() {}, nil)
		locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		svc := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return nil })))
	})
	t.Run("fn_error_and_success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, ctx, func() {}, nil).Times(2)
		locker.EXPECT().Unlock(gomock.Any()).Return(true, nil).Times(2)
		svc := &ExptResultServiceImpl{resultPublishLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return errors.New("fn") })))
		require.NoError(t, svc.withFilterItemLock(ctx, 1, 9, func(context.Context) error { return nil }))
	})
}

func TestFilterCandidateItemsUnit(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	svc := ExptResultServiceImpl{ExptItemResultRepo: items}
	got, err := svc.filterCandidateItems(context.Background(), 3, 1, []int64{9, 9, 4})
	require.NoError(t, err)
	require.Equal(t, []int64{4, 9}, got)

	items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
	_, err = svc.filterCandidateItems(context.Background(), 3, 1, nil)
	require.Error(t, err)

	items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), gomock.Any(), false).Return([]*entity.ExptItemResult{{ItemID: 9}, nil, {ItemID: 4}}, int64(0), nil)
	got, err = svc.filterCandidateItems(context.Background(), 3, 1, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{4, 9}, got)
}

func TestUpsertExptTurnResultFilterUnit(t *testing.T) {
	t.Run("invalid_scope", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		require.Error(t, svc.UpsertExptTurnResultFilter(context.Background(), 0, 1, nil))
		require.Error(t, svc.UpsertExptTurnResultFilter(context.Background(), 3, 0, nil))
	})
	t.Run("cancelled", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, errno.NeedMQRetry(svc.UpsertExptTurnResultFilter(cancelled, 3, 1, nil)))
	})
	t.Run("legacy_build_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		items := repomocks.NewMockIExptItemResultRepo(ctrl)
		turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
		turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{9}, gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
		svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns}
		require.Error(t, svc.UpsertExptTurnResultFilter(context.Background(), 3, 1, []int64{9}))
	})
	t.Run("locked_build_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		items := repomocks.NewMockIExptItemResultRepo(ctrl)
		turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
		svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns, resultPublishLocker: func() lock.ILocker { return locker }}
		require.Error(t, svc.UpsertExptTurnResultFilter(context.Background(), 3, 1, []int64{9}))
	})
	t.Run("cancel_during_item", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		items := repomocks.NewMockIExptItemResultRepo(ctrl)
		locker := lockmocks.NewMockILocker(ctrl)
		ctx, cancel := context.WithCancel(context.Background())
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ string, _ any, _ any) (bool, context.Context, func(), error) {
			cancel()
			return true, ctx, func() {}, nil
		})
		locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		svc := ExptResultServiceImpl{ExptItemResultRepo: items, resultPublishLocker: func() lock.ILocker { return locker }}
		require.True(t, errno.NeedMQRetry(svc.UpsertExptTurnResultFilter(ctx, 3, 1, []int64{9})))
	})
}

func TestCompareExptTurnResultFiltersUnit(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		svc := ExptResultServiceImpl{}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, errno.NeedMQRetry(svc.CompareExptTurnResultFilters(cancelled, 3, 1, nil, 0)))
	})
	t.Run("candidate_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		items := repomocks.NewMockIExptItemResultRepo(ctrl)
		items.EXPECT().ListItemResultsByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
		locker := lockmocks.NewMockILocker(ctrl)
		svc := ExptResultServiceImpl{ExptItemResultRepo: items, resultPublishLocker: func() lock.ILocker { return locker }}
		require.Error(t, svc.CompareExptTurnResultFilters(context.Background(), 3, 1, nil, 0))
	})
	t.Run("locked_build_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		items := repomocks.NewMockIExptItemResultRepo(ctrl)
		turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, context.Background(), func() {}, nil)
		locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
		svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns, resultPublishLocker: func() lock.ILocker { return locker }}
		require.Error(t, svc.CompareExptTurnResultFilters(context.Background(), 3, 1, []int64{9}, 0))
	})
}

func TestReconcileManualRetryFilterUnit(t *testing.T) {
	validManual := `{"run_id":2,"request_id":"req","mode":2}`
	newSvc := func(t *testing.T) (*ExptResultServiceImpl, *filterManualRepo, *repomocks.MockIExptResultPublishRepo, *repomocks.MockIExptTurnResultFilterRepo, *repomocks.MockIExptTurnResultRepo) {
		ctrl := gomock.NewController(t)
		return &ExptResultServiceImpl{}, &filterManualRepo{MockIExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl), MockIExptManualRetryFilterRepo: repomocks.NewMockIExptManualRetryFilterRepo(ctrl)}, repomocks.NewMockIExptResultPublishRepo(ctrl), repomocks.NewMockIExptTurnResultFilterRepo(ctrl), repomocks.NewMockIExptTurnResultRepo(ctrl)
	}
	t.Run("repo_not_scanner", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := ExptResultServiceImpl{ExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl)}
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("scan_error", func(t *testing.T) {
		svc, items, _, _, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), int64(3), int64(1), int64(2), int64(0), 200).Return(nil, int64(0), errors.New("scan"))
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("cursor_stalled", func(t *testing.T) {
		svc, items, _, _, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(0), 200).Return([]int64{9}, int64(5), nil)
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(5), 200).Return([]int64{9}, int64(5), nil)
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("deps_missing", func(t *testing.T) {
		svc, items, _, _, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("source_read_error", func(t *testing.T) {
		svc, items, pub, filters, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		svc.resultPublishRepo = pub
		svc.exptTurnResultFilterRepo = filters
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
		pub.EXPECT().GetItemResultPublishSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(nil, errors.New("read"))
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("source_missing", func(t *testing.T) {
		svc, items, pub, filters, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		svc.resultPublishRepo = pub
		svc.exptTurnResultFilterRepo = filters
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
		pub.EXPECT().GetItemResultPublishSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("parse_error", func(t *testing.T) {
		svc, items, pub, filters, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		svc.resultPublishRepo = pub
		svc.exptTurnResultFilterRepo = filters
		source := publishUnitSource()
		source.Item.Ext = map[string]string{entity.ManualRetryExtKey: "{bad"}
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
		pub.EXPECT().GetItemResultPublishSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("request_changed", func(t *testing.T) {
		svc, items, pub, filters, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		svc.resultPublishRepo = pub
		svc.exptTurnResultFilterRepo = filters
		source := publishUnitSource()
		source.Item.Ext = map[string]string{entity.ManualRetryExtKey: validManual}
		source.Source.Current.RequestID = "other"
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
		pub.EXPECT().GetItemResultPublishSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("publication_pending", func(t *testing.T) {
		svc, items, pub, filters, _ := newSvc(t)
		svc.ExptItemResultRepo = items
		svc.resultPublishRepo = pub
		svc.exptTurnResultFilterRepo = filters
		source := publishUnitSource()
		source.Item.Ext = map[string]string{entity.ManualRetryExtKey: validManual}
		source.Source.Current.RequestID = "req"
		source.Item.Status = entity.ItemRunState_Processing
		items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
		pub.EXPECT().GetItemResultPublishSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
		_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("build_error_then_empty", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			setup func(*repomocks.MockIExptTurnResultRepo)
		}{
			{"error", func(turns *repomocks.MockIExptTurnResultRepo) {
				turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
			}},
			{"empty", func(turns *repomocks.MockIExptTurnResultRepo) {
				turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), nil)
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				svc, items, pub, filters, turns := newSvc(t)
				svc.ExptItemResultRepo = items
				svc.resultPublishRepo = pub
				svc.exptTurnResultFilterRepo = filters
				svc.ExptTurnResultRepo = turns
				source := publishUnitSource()
				source.Item.Ext = map[string]string{entity.ManualRetryExtKey: validManual}
				source.Source.Current.RequestID = "req"
				source.ItemLog.ResultState = int32(entity.ExptItemResultStateResulted)
				items.MockIExptManualRetryFilterRepo.EXPECT().ScanManualRetryFilterItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), 200).Return([]int64{9}, int64(0), nil)
				pub.EXPECT().GetItemResultPublishSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(source, nil)
				tc.setup(turns)
				_, err := svc.ReconcileManualRetryFilterBeforeFinish(context.Background(), 3, 1, 2)
				require.True(t, errno.NeedMQRetry(err))
			})
		}
	})
}

func TestMaterializedFilterCompareUnit(t *testing.T) {
	row := func(mutate func(*entity.ExptTurnResultFilterEntity)) *entity.ExptTurnResultFilterEntity {
		r := &entity.ExptTurnResultFilterEntity{SpaceID: 3, ExptID: 1, ItemID: 9, ItemVersionID: 7, ItemIdx: 1, TurnID: 0, Status: entity.ItemRunState_Success, EvalTargetData: map[string]string{"actual_output": "out"}, EvaluatorScore: map[string]float64{"a": 1}}
		if mutate != nil {
			mutate(r)
		}
		return r
	}
	expected := func() []*entity.ExptTurnResultFilterEntity { return []*entity.ExptTurnResultFilterEntity{row(nil)} }
	ctx := context.Background()

	t.Run("empty_expected", func(t *testing.T) {
		var svc ExptResultServiceImpl
		ok, err := svc.compareMaterializedFilterLocked(ctx, 3, 1, 9, nil)
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("read_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
		filters.EXPECT().GetByExptIDItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("read"))
		svc := ExptResultServiceImpl{exptTurnResultFilterRepo: filters}
		_, err := svc.compareMaterializedFilterLocked(ctx, 3, 1, 9, expected())
		require.Error(t, err)
	})
	t.Run("len_mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
		filters.EXPECT().GetByExptIDItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		svc := ExptResultServiceImpl{exptTurnResultFilterRepo: filters}
		ok, err := svc.compareMaterializedFilterLocked(ctx, 3, 1, 9, expected())
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("legacy_partial_read_is_not_equal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
		filters.EXPECT().GetByExptIDItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterEntity{row(nil)}, nil)
		svc := ExptResultServiceImpl{exptTurnResultFilterRepo: filters}
		ok, err := svc.compareMaterializedFilterLocked(ctx, 3, 1, 9, expected())
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("full_materialized_match", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		filters := &materializedFilterRepo{MockIExptTurnResultFilterRepo: repomocks.NewMockIExptTurnResultFilterRepo(ctrl), MockIExptMaterializedFilterRepo: repomocks.NewMockIExptMaterializedFilterRepo(ctrl)}
		filters.MockIExptMaterializedFilterRepo.EXPECT().GetMaterializedByExptIDItemIDs(gomock.Any(), "3", "1", gomock.Any(), []string{"9"}).Return([]*entity.ExptTurnResultFilterEntity{row(nil)}, nil)
		svc := ExptResultServiceImpl{exptTurnResultFilterRepo: filters}
		ok, err := svc.compareMaterializedFilterLocked(ctx, 3, 1, 9, expected())
		require.NoError(t, err)
		require.True(t, ok)
	})
	t.Run("full_materialized_field_mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		filters := &materializedFilterRepo{MockIExptTurnResultFilterRepo: repomocks.NewMockIExptTurnResultFilterRepo(ctrl), MockIExptMaterializedFilterRepo: repomocks.NewMockIExptMaterializedFilterRepo(ctrl)}
		filters.MockIExptMaterializedFilterRepo.EXPECT().GetMaterializedByExptIDItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ExptTurnResultFilterEntity{row(func(r *entity.ExptTurnResultFilterEntity) { r.ItemIdx = 2 })}, nil)
		svc := ExptResultServiceImpl{exptTurnResultFilterRepo: filters}
		ok, err := svc.compareMaterializedFilterLocked(ctx, 3, 1, 9, expected())
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("read_cancelled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := ExptResultServiceImpl{exptTurnResultFilterRepo: repomocks.NewMockIExptTurnResultFilterRepo(ctrl)}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, _, err := svc.readMaterializedItemFilterLocked(cancelled, 3, 1, 9, time.Now())
		require.Error(t, err)
	})
	t.Run("materialized_filter_matches_build_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
		turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), errors.New("list"))
		svc := ExptResultServiceImpl{ExptTurnResultRepo: turns}
		_, err := svc.materializedFilterMatchesLocked(ctx, 3, 1, 9)
		require.Error(t, err)
	})
}

func TestReconcileManualRetryFilterWrapperUnit(t *testing.T) {
	t.Run("panic_recovered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := svcmocks.NewMockExptManualRetryFilterService(ctrl)
		svc.EXPECT().ReconcileManualRetryFilterBeforeFinish(gomock.Any(), int64(3), int64(1), int64(2)).DoAndReturn(func(context.Context, int64, int64, int64) (bool, error) {
			panic("boom")
		})
		ready, err := reconcileManualRetryFilterBeforeFinish(context.Background(), svc, 3, 1, 2)
		require.False(t, ready)
		require.Error(t, err)
	})
	t.Run("success_and_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		svc := svcmocks.NewMockExptManualRetryFilterService(ctrl)
		svc.EXPECT().ReconcileManualRetryFilterBeforeFinish(gomock.Any(), int64(3), int64(1), int64(2)).Return(true, nil)
		svc.EXPECT().ReconcileManualRetryFilterBeforeFinish(gomock.Any(), int64(3), int64(1), int64(2)).Return(false, errors.New("f"))
		ready, err := reconcileManualRetryFilterBeforeFinish(context.Background(), svc, 3, 1, 2)
		require.True(t, ready)
		require.NoError(t, err)
		ready, err = reconcileManualRetryFilterBeforeFinish(context.Background(), svc, 3, 1, 2)
		require.False(t, ready)
		require.Error(t, err)
	})
}

func TestPreparePublicationRemainingBranches(t *testing.T) {
	expt := func() *entity.Experiment { return publishUnitExperiment() }

	t.Run("multiset_target_space", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].TargetResultID = 88
		source.ItemRef = &entity.ExptItemRef{ItemConfig: &entity.ExptItemConfig{TargetSourceSpaceID: 4}}
		e := expt()
		e.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		d.targets.EXPECT().GetRecordByID(gomock.Any(), int64(4), int64(88)).Return(&entity.EvalTargetRecord{ID: 88, SpaceID: 4, ItemID: 9, TurnID: 0, ExperimentRunID: 2, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
		publication, err := d.svc.PrepareItemResultPublication(context.Background(), source, e)
		require.NoError(t, err)
		require.Len(t, publication.Turns, 1)
	})
	t.Run("frozen_turn_missing_same_len", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns = map[int64]*entity.ExptTurnResultRunLog{1: publishUnitLog(1)}
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, expt())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("evaluator_ref_incomplete", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 0}}}
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, expt())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("batch_get_cancelled", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		ctx, cancel := context.WithCancel(context.Background())
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{61}, false, false).DoAndReturn(func(context.Context, []int64, bool, bool) ([]*entity.EvaluatorRecord, error) {
			cancel()
			return []*entity.EvaluatorRecord{{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, EvaluatorVersionID: 51, Alias: "a", Status: entity.EvaluatorRunStatusSuccess}}, nil
		})
		_, err := d.svc.PrepareItemResultPublication(ctx, source, expt())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("idgen_invalid", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "a", RecordID: 61}}}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.EvaluatorRecord{{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, EvaluatorVersionID: 51, Alias: "a", Status: entity.EvaluatorRunStatusSuccess}}, nil)
		d.idgen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{0}, nil)
		_, err := d.svc.PrepareItemResultPublication(context.Background(), source, expt())
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("sort_variants", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		source := publishUnitSource()
		source.Source.Turns[0].EvaluatorResultIds = &entity.EvaluatorResults{
			Registered: []*entity.RegisteredEvalResult{{VersionID: 51, Alias: "b", RecordID: 61}, {VersionID: 51, Alias: "a", RecordID: 62}},
			Inline:     []*entity.InlineEvalResult{{InlineKey: "z", RecordID: 63}, {InlineKey: "a", RecordID: 64}},
		}
		d.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{61, 62, 63, 64}, false, false).Return([]*entity.EvaluatorRecord{
			{ID: 61, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, EvaluatorVersionID: 51, Alias: "b", Status: entity.EvaluatorRunStatusSuccess},
			{ID: 62, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, EvaluatorVersionID: 51, Alias: "a", Status: entity.EvaluatorRunStatusSuccess},
			{ID: 63, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "z", Status: entity.EvaluatorRunStatusSuccess},
			{ID: 64, SpaceID: 3, ExperimentID: 1, ExperimentRunID: 2, ItemID: 9, TurnID: 0, SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "a", Status: entity.EvaluatorRunStatusSuccess},
		}, nil)
		d.idgen.EXPECT().GenMultiIDs(gomock.Any(), 4).Return([]int64{901, 902, 903, 904}, nil)
		publication, err := d.svc.PrepareItemResultPublication(context.Background(), source, expt())
		require.NoError(t, err)
		require.Len(t, publication.Refs, 4)
	})
}

func TestPublishPreparedRemainingBranches(t *testing.T) {
	t.Run("source_check_cancels", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		lockCtx, cancel := context.WithCancel(context.Background())
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, lockCtx, func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptItemResultPublishSource) (bool, error) {
			cancel()
			return true, nil
		})
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, nil)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("before_commit_cancels", func(t *testing.T) {
		d := newPublishUnitDeps(t)
		lockCtx, cancel := context.WithCancel(context.Background())
		d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, lockCtx, func() {}, nil)
		d.locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		d.repo.EXPECT().IsItemResultPublishSourceCurrent(gomock.Any(), gomock.Any()).Return(true, nil)
		_, err := d.svc.PublishPreparedItemResult(context.Background(), &entity.ExptItemResultPublication{Source: publishUnitSource()}, func(context.Context, *entity.ExptItemResultPublishSource) error {
			cancel()
			return nil
		})
		require.True(t, errno.NeedMQRetry(err))
	})
}

func TestRecordCurrentItemResultsRemainingBranches(t *testing.T) {
	event := func() *entity.ExptScheduleEvent {
		return &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}
	}
	expt := publishUnitExperiment()
	t.Run("legacy_source_identity_mismatch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		scheduler := &ExptSchedulerImpl{}
		svc := svcmocks.NewMockExptResultPublicationService(ctrl)
		source := publishUnitSource()
		source.Source.Current.RequestID = "req"
		svc.EXPECT().GetItemResultPublishSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).Return(source, nil)
		scanned := currentUnitSource()
		scanned.Current.RequestID = "other"
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Success, Source: scanned}
		require.NoError(t, scheduler.recordCurrentItemResults(context.Background(), event(), []*entity.ExptEvalItem{item}, entitymocks.NewMockExptSchedulerMode(ctrl), expt, svc))
	})
	t.Run("cancel_after_applied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		d := newPublishUnitDeps(t)
		_ = d
		scheduler := &ExptSchedulerImpl{}
		svc := svcmocks.NewMockExptResultPublicationService(ctrl)
		mode := entitymocks.NewMockExptSchedulerMode(ctrl)
		source := publishUnitSource()
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Success, Publication: source}
		ctx, cancel := context.WithCancel(context.Background())
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), source, expt).Return(&entity.ExptItemResultPublication{Source: source}, nil)
		svc.EXPECT().PublishPreparedItemResult(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptItemResultPublication, func(context.Context, *entity.ExptItemResultPublishSource) error) (bool, error) {
			cancel()
			return true, nil
		})
		require.True(t, errno.NeedMQRetry(scheduler.recordCurrentItemResults(ctx, event(), []*entity.ExptEvalItem{item}, mode, expt, svc)))
	})
	t.Run("cancel_during_publish_result", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		scheduler := &ExptSchedulerImpl{}
		svc := svcmocks.NewMockExptResultPublicationService(ctrl)
		mode := entitymocks.NewMockExptSchedulerMode(ctrl)
		source := publishUnitSource()
		item := &entity.ExptEvalItem{ItemID: 9, RequestID: "req", State: entity.ItemRunState_Success, Publication: source}
		ctx, cancel := context.WithCancel(context.Background())
		svc.EXPECT().PrepareItemResultPublication(gomock.Any(), source, expt).Return(&entity.ExptItemResultPublication{Source: source}, nil)
		svc.EXPECT().PublishPreparedItemResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
		mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, []*entity.ExptTurnEvaluatorResultRef, *entity.ExptScheduleEvent) error {
			cancel()
			return nil
		})
		require.True(t, errno.NeedMQRetry(scheduler.recordCurrentItemResults(ctx, event(), []*entity.ExptEvalItem{item}, mode, expt, svc)))
	})
}
