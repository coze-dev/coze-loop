// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type hookManagementTemplateRepo struct {
	repo.IExptTemplateRepo
	scheduled, duplicate bool
	calls                int
	original             *entity.ExptTemplate
}

func (r *hookManagementTemplateRepo) GetByID(context.Context, int64, *int64) (*entity.ExptTemplate, error) {
	if r.original != nil {
		return r.original, nil
	}
	return &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "old"}, ExptInfo: &entity.ExptInfo{CronActivate: r.scheduled}}, nil
}

func TestHookManagementTemplateOmittedRefs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		items    []*entity.EvaluatorIDVersionItem
		want     []int64
		metadata bool
	}{
		{name: "nil_hook_only", want: []int64{201}},
		{name: "nil_hook_and_metadata", want: []int64{201}, metadata: true},
		{name: "explicit_empty", items: []*entity.EvaluatorIDVersionItem{}, want: []int64{}},
		{name: "explicit_replace", items: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 102, EvaluatorVersionID: 202}}, want: []int64{202}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &entity.ExptTemplate{
				Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10, Name: "old", Desc: "keep description"},
				TripleConfig: &entity.ExptTemplateTuple{EvalSetID: 11, EvalSetVersionID: 12, TargetID: 13, TargetVersionID: 14,
					EvaluatorIDVersionItems: []*entity.EvaluatorIDVersionItem{{EvaluatorID: 101, EvaluatorVersionID: 201, Version: "v1", ScoreWeight: 0.7}},
					EvaluatorVersionIds:     []int64{201}},
				EvaluatorVersionRef: []*entity.ExptTemplateEvaluatorVersionRef{{EvaluatorID: 101, EvaluatorVersionID: 201}},
			}
			old := &hookManagementTemplateRepo{original: original}
			base := &ExptTemplateManagerImpl{templateRepo: old, idgen: hookCreateIDs{}}
			stop := errors.New("validated write boundary")
			capture := &hookManagementCapture{err: stop}
			manager, err := WithExptTemplateHookConfigUpdate(base, capture, hookManagementOwner(hookcomponent.ConfigOwnerTemplate), hookManagementInput())
			require.NoError(t, err)
			param := &entity.UpdateExptTemplateParam{TemplateID: 20, SpaceID: 10, EvaluatorIDVersionItems: tc.items}
			if tc.metadata {
				param.Name = "new"
				param.Description = "new description"
			}
			_, err = manager.Update(context.Background(), param, &entity.Session{UserID: "authorized"})
			require.ErrorIs(t, err, stop)
			require.NotNil(t, capture.template)
			require.Equal(t, tc.want, capture.template.TripleConfig.EvaluatorVersionIds)
			refVersions := make([]int64, 0, len(capture.refs))
			for _, ref := range capture.refs {
				refVersions = append(refVersions, ref.EvaluatorVersionID)
			}
			require.Equal(t, tc.want, refVersions, "refs and TripleConfig must describe the same selection")
			require.Equal(t, int64(11), capture.template.TripleConfig.EvalSetID)
			require.Equal(t, int64(12), capture.template.TripleConfig.EvalSetVersionID)
			require.Equal(t, int64(13), capture.template.TripleConfig.TargetID)
			require.Equal(t, int64(14), capture.template.TripleConfig.TargetVersionID)
			require.Zero(t, old.calls)
			if tc.items == nil {
				require.Nil(t, param.EvaluatorIDVersionItems, "request input must stay omitted")
				require.NotSame(t, original.TripleConfig.EvaluatorIDVersionItems[0], capture.template.TripleConfig.EvaluatorIDVersionItems[0])
				require.Equal(t, "v1", capture.template.TripleConfig.EvaluatorIDVersionItems[0].Version)
				require.Equal(t, 0.7, capture.template.TripleConfig.EvaluatorIDVersionItems[0].ScoreWeight)
			}
			require.Equal(t, []int64{201}, original.TripleConfig.EvaluatorVersionIds)
			require.Equal(t, int64(201), original.EvaluatorVersionRef[0].EvaluatorVersionID)
		})
	}
}
func (r *hookManagementTemplateRepo) GetByName(context.Context, string, int64, entity.ExptType) (*entity.ExptTemplate, bool, error) {
	return nil, r.duplicate, nil
}
func (r *hookManagementTemplateRepo) UpdateWithRefs(context.Context, *entity.ExptTemplate, []*entity.ExptTemplateEvaluatorRef) error {
	r.calls++
	return errors.New("legacy write must not happen")
}
func TestHookManagementTemplatePreservesValidation(t *testing.T) {
	for _, mode := range []string{"valid", "duplicate", "scheduled"} {
		t.Run(mode, func(t *testing.T) {
			old := &hookManagementTemplateRepo{scheduled: mode == "scheduled", duplicate: mode == "duplicate"}
			base := &ExptTemplateManagerImpl{templateRepo: old}
			stop := errors.New("stop after original validation")
			capture := &hookManagementCapture{err: stop}
			manager, err := WithExptTemplateHookConfigUpdate(base, capture, hookManagementOwner(hookcomponent.ConfigOwnerTemplate), hookManagementInput())
			require.NoError(t, err)
			_, err = manager.Update(context.Background(), &entity.UpdateExptTemplateParam{TemplateID: 20, SpaceID: 10, Name: "new", Description: "new description"}, &entity.Session{UserID: "authorized"})
			require.Zero(t, old.calls)
			require.Same(t, old, base.templateRepo)
			switch mode {
			case "valid":
				require.ErrorIs(t, err, stop)
				require.NotNil(t, capture.template)
				require.Equal(t, "new description", capture.template.Meta.Desc)
			case "scheduled":
				require.ErrorIs(t, err, repo.ErrHookConfigScheduleBindingRequired)
				require.Nil(t, capture.template)
			default:
				require.Error(t, err)
				require.Nil(t, capture.template)
			}
		})
	}
}
