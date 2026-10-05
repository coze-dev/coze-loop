// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type hookCreateIDs struct {
	idgen.IIDGenerator
	err error
}

func (g hookCreateIDs) GenID(context.Context) (int64, error) { return 20, g.err }

func (g hookCreateIDs) GenMultiIDs(_ context.Context, n int) ([]int64, error) {
	if g.err != nil {
		return nil, g.err
	}
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = int64(101 + i)
	}
	return ids, nil
}

type hookCreateLegacyRepo struct {
	repo.IExperimentRepo
	calls int
}

func (r *hookCreateLegacyRepo) Create(context.Context, *entity.Experiment, []*entity.ExptEvaluatorRef) error {
	r.calls++
	return nil
}

type hookCreateCapture struct {
	repo.IHookConfigRepo
	expt         *entity.Experiment
	template     *entity.ExptTemplate
	exptRefs     []*entity.ExptEvaluatorRef
	templateRefs []*entity.ExptTemplateEvaluatorRef
	in           repo.HookConfigCreateInput
	err          error
}

func (r *hookCreateCapture) CreateExperimentWithHookConfig(_ context.Context, expt *entity.Experiment, refs []*entity.ExptEvaluatorRef, in repo.HookConfigCreateInput) error {
	r.expt, r.exptRefs, r.in = expt, refs, in
	return r.err
}
func (r *hookCreateCapture) CreateTemplateWithHookConfig(_ context.Context, template *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef, in repo.HookConfigCreateInput) error {
	r.template, r.templateRefs, r.in = template, refs, in
	return r.err
}
func hookCreateInput() repo.HookConfigCreateInput {
	return repo.HookConfigCreateInput{WorkspaceID: 10, ExecutionScope: "local", KeyID: "key",
		Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"keep":7}`)}}}
}

func TestHookConfigCreateManagerIsolatedAndDisabled(t *testing.T) {
	baseRepo := &hookCreateLegacyRepo{}
	base := &ExptMangerImpl{exptRepo: baseRepo, idgenerator: hookCreateIDs{}}
	capture := &hookCreateCapture{}
	in := hookCreateInput()
	scoped, err := WithExptHookConfigCreate(base, capture, in)
	require.NoError(t, err)
	expt := &entity.Experiment{ID: 20, SpaceID: 10, Name: "new"}
	refs := []*entity.ExptEvaluatorRef{{ExptID: 20, SpaceID: 10, EvaluatorVersionID: 30}}
	*in.Config.Before.ParametersJSON = `{"mutated":true}`
	err = scoped.(*ExptMangerImpl).exptRepo.Create(context.Background(), expt, refs)
	require.NoError(t, err)
	require.NotNil(t, capture.expt, "explicit disabled configuration must route to encrypted creator")
	require.Same(t, expt, capture.expt)
	require.Equal(t, int64(101), capture.exptRefs[0].ID)
	require.JSONEq(t, `{"keep":7}`, *capture.in.Config.Before.ParametersJSON)
	require.False(t, *capture.in.Config.Before.Enabled)
	require.Same(t, baseRepo, base.exptRepo)
	require.Zero(t, baseRepo.calls)
	require.NoError(t, base.exptRepo.Create(context.Background(), expt, nil))
	require.Equal(t, 1, baseRepo.calls)
}

func TestHookConfigCreateManagerNilIsLegacy(t *testing.T) {
	base := &ExptMangerImpl{}
	got, err := WithExptHookConfigCreate(base, nil, repo.HookConfigCreateInput{})
	require.NoError(t, err)
	require.Same(t, base, got)
}

func TestHookConfigCreateManagerFailureIsClosed(t *testing.T) {
	base := &ExptMangerImpl{exptRepo: &hookCreateLegacyRepo{}, idgenerator: hookCreateIDs{}}
	got, err := WithExptHookConfigCreate(base, nil, hookCreateInput())
	require.Error(t, err)
	require.Nil(t, got)
	sentinel := errors.New("id allocation failed")
	base.idgenerator = hookCreateIDs{err: sentinel}
	capture := &hookCreateCapture{}
	got, err = WithExptHookConfigCreate(base, capture, hookCreateInput())
	require.NoError(t, err)
	err = got.(*ExptMangerImpl).exptRepo.Create(context.Background(), &entity.Experiment{ID: 20, SpaceID: 10}, []*entity.ExptEvaluatorRef{{ExptID: 20, SpaceID: 10}})
	require.ErrorIs(t, err, sentinel)
	require.Nil(t, capture.expt)
}

type hookCreateCloneRepo struct{ repo.IExperimentRepo }

func (r hookCreateCloneRepo) GetByID(_ context.Context, id, space int64) (*entity.Experiment, error) {
	return &entity.Experiment{ID: id, SpaceID: space, Name: "clone", EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{{EvaluatorID: 41, EvaluatorVersionID: 42}}}, nil
}
func (r hookCreateCloneRepo) GetByName(context.Context, string, int64) (*entity.Experiment, bool, error) {
	return nil, false, nil
}

type hookCreateWriteTracker struct {
	platestwrite.ILatestWriteTracker
	id int64
}

func (w *hookCreateWriteTracker) SetWriteFlag(_ context.Context, _ platestwrite.ResourceType, id int64, _ ...platestwrite.SetWriteFlagOpt) {
	w.id = id
}

func TestHookConfigCreateCloneUsesGeneratedID(t *testing.T) {
	tracker := &hookCreateWriteTracker{}
	base := &ExptMangerImpl{exptRepo: hookCreateCloneRepo{}, idgenerator: hookCreateIDs{}, lwt: tracker}
	capture := &hookCreateCapture{}
	scoped, err := WithExptHookConfigCreate(base, capture, hookCreateInput())
	require.NoError(t, err)
	got, err := scoped.Clone(context.Background(), 19, 10, &entity.Session{UserID: "authorized"})
	require.NoError(t, err)
	require.Equal(t, int64(20), got.ID)
	require.Same(t, got, capture.expt)
	require.Len(t, capture.exptRefs, 1)
	require.Equal(t, int64(20), capture.exptRefs[0].ExptID)
	require.Equal(t, int64(101), capture.exptRefs[0].ID)
	require.Equal(t, int64(20), tracker.id)
}
