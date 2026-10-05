// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type batchDeleteManagerPort struct {
	calls   int
	err     error
	deleted []*entity.Experiment
}

func (p *batchDeleteManagerPort) HookDeletionScope() string { return "scope" }
func (p *batchDeleteManagerPort) DeleteExperiments(context.Context, []int64, int64, string) ([]*entity.Experiment, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.deleted, nil
}

type batchDeleteManagerReads struct {
	repo.IExperimentRepo
	rows          []*entity.Experiment
	legacyDeletes int
}

func (r *batchDeleteManagerReads) MGetByID(context.Context, []int64, int64) ([]*entity.Experiment, error) {
	return r.rows, nil
}
func (r *batchDeleteManagerReads) MDelete(context.Context, []int64, int64) error {
	r.legacyDeletes++
	return nil
}

type batchDeleteRunOwner struct {
	repo.IHookRepo
	key entity.HookRunKey
}

func (r *batchDeleteRunOwner) HookExecutionBinding() (entity.HookRunKey, string, string) {
	return r.key, "scope", ""
}

type batchDeleteFinalization struct{ repo.IHookFinalizationRepo }
type batchDeleteTemplateEffect struct {
	IExptTemplateManager
	statuses []entity.ExptStatus
	deltas   []int64
}

func (t *batchDeleteTemplateEffect) UpdateExptInfo(_ context.Context, _, _, _ int64, status entity.ExptStatus, delta int64, _ *int64) error {
	t.statuses = append(t.statuses, status)
	t.deltas = append(t.deltas, delta)
	return nil
}

func TestHookBatchDeletionManagerPureAtomicPortAndOriginalEffects(t *testing.T) {
	base := newTestExptManager(gomock.NewController(t))
	base.finalization = &ExptManagerFinalizationDependencies{ExecutionScope: "scope", Runs: &batchDeleteRunOwner{}, Repository: &batchDeleteFinalization{}}
	old := &batchDeleteManagerPort{}
	base.deletion = old
	values := []*entity.Experiment{{ID: 20, SpaceID: 1, Status: entity.ExptStatus_Success, ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 70}}, {ID: 21, SpaceID: 1, Status: entity.ExptStatus_Failed, ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 71}}}
	reads := &batchDeleteManagerReads{rows: values}
	base.exptRepo = reads
	templates := &batchDeleteTemplateEffect{}
	base.templateManager = templates
	port := &batchDeleteManagerPort{deleted: values, err: errors.New("atomic rollback")}
	m, err := NewExptManagerForHookDeletion(base, port, "scope")
	require.NoError(t, err)
	require.ErrorIs(t, m.MDelete(context.Background(), []int64{20, 21}, 1, &entity.Session{UserID: "actor"}), port.err)
	require.Empty(t, templates.statuses)
	require.Equal(t, 1, port.calls)
	require.Zero(t, reads.legacyDeletes)
	require.Same(t, old, base.deletion)
	port.err = nil
	require.NoError(t, m.MDelete(context.Background(), []int64{20, 21}, 1, &entity.Session{UserID: "actor"}))
	require.Equal(t, 2, port.calls)
	require.Equal(t, []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed}, templates.statuses)
	require.Equal(t, []int64{-1, -1}, templates.deltas)
}

func TestHookBatchDeletionManagerPureRejectsSingleRunAndForeignScope(t *testing.T) {
	base := newTestExptManager(gomock.NewController(t))
	owner := &batchDeleteRunOwner{key: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 20, RunID: 31}}
	base.finalization = &ExptManagerFinalizationDependencies{ExecutionScope: "scope", Runs: owner, Repository: &batchDeleteFinalization{}}
	_, err := NewExptManagerForHookDeletion(base, &batchDeleteManagerPort{}, "scope")
	require.Error(t, err)
	owner.key = entity.HookRunKey{}
	_, err = NewExptManagerForHookDeletion(base, &batchDeleteManagerPort{}, "foreign")
	require.Error(t, err)
}
