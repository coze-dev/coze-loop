// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type hookManagementCapture struct {
	repo.IHookConfigRepo
	expt     *entity.Experiment
	template *entity.ExptTemplate
	refs     []*entity.ExptTemplateEvaluatorRef
	in       entity.HookConfigUpdateInput
	owner    hookcomponent.ConfigOwner
	err      error
}

func (c *hookManagementCapture) UpdateExperimentWithHookConfig(_ context.Context, owner hookcomponent.ConfigOwner, e *entity.Experiment, in entity.HookConfigUpdateInput) error {
	c.expt, c.owner, c.in = e, owner, in
	return c.err
}
func (c *hookManagementCapture) UpdateTemplateWithHookConfig(_ context.Context, owner hookcomponent.ConfigOwner, e *entity.ExptTemplate, refs []*entity.ExptTemplateEvaluatorRef, in entity.HookConfigUpdateInput) error {
	c.template, c.owner, c.refs, c.in = e, owner, refs, in
	return c.err
}

type hookManagementAudit struct {
	calls  int
	reject bool
}

func (a *hookManagementAudit) Audit(_ context.Context, _ audit.AuditParam) (audit.AuditRecord, error) {
	a.calls++
	status := audit.AuditStatus_Approved
	if a.reject {
		status = audit.AuditStatus_Rejected
	}
	return audit.AuditRecord{AuditStatus: status}, nil
}

type hookManagementLegacyRepo struct {
	repo.IExperimentRepo
	calls int
}

func (r *hookManagementLegacyRepo) Update(context.Context, *entity.Experiment) error {
	r.calls++
	return nil
}
func hookManagementOwner(kind hookcomponent.ConfigOwnerKind) hookcomponent.ConfigOwner {
	return hookcomponent.ConfigOwner{WorkspaceID: 10, ObjectID: 20, ExecutionScope: "local", Kind: kind}
}
func hookManagementInput() entity.HookConfigUpdateInput {
	return entity.HookConfigUpdateInput{KeyID: "key", ExpectedRevision: "revision", Config: &entity.LifecycleHookConf{After: &entity.HookConfig{Enabled: gptr.Of(false), ParametersJSON: gptr.Of(`{"kept":7}`)}}}
}
func TestHookManagementExperimentUsesOriginalAudit(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "approved", true: "rejected"}[reject], func(t *testing.T) {
			old := &hookManagementLegacyRepo{}
			a := &hookManagementAudit{reject: reject}
			base := &ExptMangerImpl{exptRepo: old, audit: a}
			capture := &hookManagementCapture{}
			in := hookManagementInput()
			manager, err := WithExptHookConfigUpdate(base, capture, hookManagementOwner(hookcomponent.ConfigOwnerExperiment), in)
			require.NoError(t, err)
			*in.Config.After.ParametersJSON = `{"changed":true}`
			meta := &entity.Experiment{ID: 20, SpaceID: 10, Name: "valid", Description: "validated metadata"}
			err = manager.Update(context.Background(), meta, &entity.Session{UserID: "authorized"})
			require.Equal(t, 1, a.calls)
			require.Zero(t, old.calls)
			require.Same(t, old, base.exptRepo)
			if reject {
				require.Error(t, err)
				require.Nil(t, capture.expt)
				return
			}
			require.NoError(t, err)
			require.Same(t, meta, capture.expt, "ordinary metadata and Hook config must reach the same updater")
			require.Equal(t, "revision", capture.in.ExpectedRevision)
			require.JSONEq(t, `{"kept":7}`, *capture.in.Config.After.ParametersJSON)
		})
	}
}
func TestHookManagementExperimentNoopAndMissingCapability(t *testing.T) {
	base := &ExptMangerImpl{}
	same, err := WithExptHookConfigUpdate(base, nil, hookcomponent.ConfigOwner{}, entity.HookConfigUpdateInput{})
	require.NoError(t, err)
	require.Same(t, base, same)
	_, err = WithExptHookConfigUpdate(base, nil, hookManagementOwner(hookcomponent.ConfigOwnerExperiment), hookManagementInput())
	require.ErrorIs(t, err, entity.ErrHookConfigStorage)
	capture := &hookManagementCapture{err: errors.New("storage stopped")}
	manager, err := WithExptHookConfigUpdate(base, capture, hookManagementOwner(hookcomponent.ConfigOwnerExperiment), hookManagementInput())
	require.NoError(t, err)
	err = manager.(*ExptMangerImpl).exptRepo.Update(context.Background(), &entity.Experiment{ID: 21, SpaceID: 10})
	require.ErrorIs(t, err, entity.ErrHookConfigStorage)
	require.Nil(t, capture.expt)
}
