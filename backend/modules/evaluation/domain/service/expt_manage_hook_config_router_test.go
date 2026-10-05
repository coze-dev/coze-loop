// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"testing"
)

type hookConfigManagerRouter struct {
	IExptManager
	base *ExptMangerImpl
}

func (r hookConfigManagerRouter) HookConfigBaseManager() *ExptMangerImpl { return r.base }

func TestHookConfigRouterCreateUnwrapPreservesAtomicWrite(t *testing.T) {
	old := &hookCreateLegacyRepo{}
	base := &ExptMangerImpl{exptRepo: old, idgenerator: hookCreateIDs{}}
	router := hookConfigManagerRouter{IExptManager: base, base: base}
	capture := &hookCreateCapture{}
	scoped, err := WithExptHookConfigCreate(router, capture, hookCreateInput())
	require.NoError(t, err)
	expt := &entity.Experiment{ID: 20, SpaceID: 10}
	require.NoError(t, scoped.(*ExptMangerImpl).exptRepo.Create(context.Background(), expt, nil))
	require.Same(t, expt, capture.expt)
	require.Zero(t, old.calls)
	require.Same(t, old, base.exptRepo)
	unchanged, err := WithExptHookConfigCreate(router, nil, repo.HookConfigCreateInput{})
	require.NoError(t, err)
	require.Equal(t, router, unchanged)
}

func TestHookConfigRouterUpdateRejectsNilUnwrap(t *testing.T) {
	_, err := WithExptHookConfigUpdate(hookConfigManagerRouter{}, nil, hookManagementOwner(hookcomponent.ConfigOwnerExperiment), hookManagementInput())
	require.ErrorIs(t, err, entity.ErrHookConfigStorage)
}

func TestHookConfigRouterUpdateUnwrapPreservesAtomicWrite(t *testing.T) {
	old := &hookManagementLegacyRepo{}
	audit := &hookManagementAudit{}
	base := &ExptMangerImpl{exptRepo: old, audit: audit}
	router := hookConfigManagerRouter{IExptManager: base, base: base}
	capture := &hookManagementCapture{}
	scoped, err := WithExptHookConfigUpdate(router, capture, hookManagementOwner(hookcomponent.ConfigOwnerExperiment), hookManagementInput())
	require.NoError(t, err)
	expt := &entity.Experiment{ID: 20, SpaceID: 10, Name: "renamed"}
	require.NoError(t, scoped.Update(context.Background(), expt, &entity.Session{UserID: "authorized"}))
	require.Same(t, expt, capture.expt)
	require.Equal(t, 1, audit.calls)
	require.Zero(t, old.calls)
	require.Same(t, old, base.exptRepo)
}
