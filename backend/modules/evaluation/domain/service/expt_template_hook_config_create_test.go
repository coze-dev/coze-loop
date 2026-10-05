// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

func TestHookConfigCreateTemplateManager(t *testing.T) {
	base := &ExptTemplateManagerImpl{idgen: hookCreateIDs{}}
	legacy, err := WithExptTemplateHookConfigCreate(base, nil, repo.HookConfigCreateInput{})
	require.NoError(t, err)
	require.Same(t, base, legacy)
	capture := &hookCreateCapture{}
	scoped, err := WithExptTemplateHookConfigCreate(base, capture, hookCreateInput())
	require.NoError(t, err)
	require.NotSame(t, base, scoped, "configuration must use a request-scoped manager")
	template := &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: 20, WorkspaceID: 10}}
	err = scoped.(*ExptTemplateManagerImpl).templateRepo.Create(context.Background(), template, []*entity.ExptTemplateEvaluatorRef{{SpaceID: 10, ExptTemplateID: 20}})
	require.NoError(t, err)
	require.Same(t, template, capture.template)
	require.Equal(t, int64(101), capture.templateRefs[0].ID)
	require.Nil(t, base.templateRepo)
}
