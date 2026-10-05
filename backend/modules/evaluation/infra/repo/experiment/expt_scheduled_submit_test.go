// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func preparedScheduledFixture(t *testing.T, f *scheduledTriggerFixture, tr *entity.ScheduledRunTrigger) *entity.PreparedScheduledExpt {
	t.Helper()
	key := entity.ExptTemplateScheduleBindingKey{SpaceID: f.b.SpaceID, TemplateID: f.b.TemplateID, ExecutionScope: f.b.ExecutionScope}
	po, refs, err := templateScheduleRows(f.sql, key)
	require.NoError(t, err)
	k := entity.HookRunKey{WorkspaceID: tr.SpaceID, ExperimentID: tr.ExperimentID, RunID: tr.RunID}
	in := &entity.PreparedScheduledExpt{
		Binding: f.b, TemplateRevision: templateScheduleRevision(po, refs), ConfigCipher: []byte("prepared-encrypted-config"),
		Experiment: &entity.Experiment{ID: tr.ExperimentID, SpaceID: tr.SpaceID, CreatedBy: f.b.UserID, Name: "scheduled", ExptTemplateMeta: &entity.ExptTemplateMeta{ID: tr.TemplateID}, Status: entity.ExptStatus_Pending, ExptType: entity.ExptType_Offline, TriggerType: "schedule"},
		Stats:      &entity.ExptStats{ID: scheduledTriggerSequence.Add(1), SpaceID: tr.SpaceID, ExptID: tr.ExperimentID},
		Refs:       []*entity.ExptEvaluatorRef{{ID: scheduledTriggerSequence.Add(1), SpaceID: tr.SpaceID, ExptID: tr.ExperimentID, EvaluatorID: 91, EvaluatorVersionID: 92}},
		Mappings:   []*entity.ExptTurnResultFilterKeyMapping{{SpaceID: tr.SpaceID, ExptID: tr.ExperimentID, FromField: "92", ToKey: "key1", FieldType: entity.FieldTypeEvaluator}},
		Run:        entity.HookCreateRunInput{Key: k, RunLog: &entity.ExptRunLog{ID: tr.RunID, ExptRunID: tr.RunID, ExptID: tr.ExperimentID, SpaceID: tr.SpaceID, CreatedBy: f.b.UserID, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Pending)}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte("prepared-encrypted-snapshot"), KeyID: "key", Hash: strings.Repeat("a", 64), ExecutionScope: f.b.ExecutionScope}, Before: &entity.HookOperationSeed{ID: scheduledTriggerSequence.Add(1), OperationID: "scheduled-before-" + f.b.BindingID, IdempotencyKey: "before-" + f.b.BindingID}},
	}
	t.Cleanup(func() {
		for _, table := range []string{"expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_stats", "expt_evaluator_ref", "expt_turn_result_filter_key_mapping"} {
			require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", tr.SpaceID, tr.ExperimentID).Delete(map[string]any{}).Error)
		}
	})
	return in
}

func TestScheduledSubmissionWriterMySQLAtomic(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	tr, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	in := preparedScheduledFixture(t, f, tr)
	w := NewScheduledExptSubmissionWriter()
	failure := errors.New("after all submission SQL")
	_, err = f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
		if err := w.Write(ctx, p, tr, user, in); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	f.counts(t, "pending", 0, 0)
	for _, table := range []string{"expt_stats", "expt_evaluator_ref", "expt_turn_result_filter_key_mapping", "expt_lifecycle_run", "expt_lifecycle_hook_run"} {
		var n int64
		require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", tr.SpaceID, tr.ExperimentID).Count(&n).Error)
		require.Zero(t, n, table)
	}
	got, err := f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
		return w.Write(ctx, p, tr, user, in)
	})
	require.NoError(t, err)
	require.Equal(t, tr.RunID, got.RunID)
	f.counts(t, "submitted", 1, 1)
	run, err := NewHookRunRepo(f.p).GetRun(ctx, in.Run.Key)
	require.NoError(t, err)
	require.Equal(t, f.b.UserID, run.CreatedBy)
	require.Equal(t, entity.HookGateWaiting, run.State.Gate)
	for _, table := range []string{"expt_stats", "expt_evaluator_ref", "expt_turn_result_filter_key_mapping", "expt_lifecycle_run", "expt_lifecycle_hook_run"} {
		var n int64
		require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", tr.SpaceID, tr.ExperimentID).Count(&n).Error)
		require.EqualValues(t, 1, n, table)
	}
}

func TestScheduledSubmissionWriterMySQLTemplateRevision(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	tr, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	in := preparedScheduledFixture(t, f, tr)
	require.NoError(t, f.sql.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", tr.TemplateID, tr.SpaceID).UpdateColumn("name", "edited").Error)
	_, err = f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
		return NewScheduledExptSubmissionWriter().Write(ctx, p, tr, user, in)
	})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	f.counts(t, "pending", 0, 0)
}

func TestScheduledSubmissionWriterReadFailure(t *testing.T) {
	p, mock := initializationDB(t)
	b := scheduleBindingFixture(t)
	b.JobID = "job-1"
	tr := entity.ScheduledRunTrigger{ID: 30, SpaceID: 10, TemplateID: 20, BindingID: b.BindingID, BindingVersion: 1, InstanceID: "instance", ExperimentID: 40, RunID: 50, Status: "pending", CreatedAt: time.Unix(100, 0)}
	in := &entity.PreparedScheduledExpt{Binding: *b, TemplateRevision: "revision", Experiment: &entity.Experiment{ID: 40, SpaceID: 10, CreatedBy: b.UserID, ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 20}, Status: entity.ExptStatus_Pending, TriggerType: "schedule"}, Stats: &entity.ExptStats{ID: 60, SpaceID: 10, ExptID: 40}, ConfigCipher: []byte("encrypted"), Run: entity.HookCreateRunInput{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 40, RunID: 50}, RunLog: &entity.ExptRunLog{ID: 50, ExptRunID: 50, ExptID: 40, SpaceID: 10, CreatedBy: b.UserID}}}
	failure := errors.New("template read failed")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM .*expt_template.*").WillReturnError(failure)
	mock.ExpectRollback()
	require.ErrorIs(t, NewScheduledExptSubmissionWriter().Write(context.Background(), p, tr, b.UserID, in), failure)
	require.NoError(t, mock.ExpectationsWereMet())
}
