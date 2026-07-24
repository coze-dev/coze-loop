// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package schedule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNoopSkillPreloader(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	preloader := NewNoopSkillPreloader()

	t.Run("implements ISkillPreloader interface", func(t *testing.T) {
		var _ rpc.ISkillPreloader = (*noopSkillPreloader)(nil)
		assert.NotNil(t, preloader)
	})

	t.Run("PreloadSkills returns nil", func(t *testing.T) {
		skillID := int64(1)
		version := "v1"
		err := preloader.PreloadSkills(ctx, 123, 456, []*entity.SkillConfig{
			{SkillID: &skillID, Version: &version},
		}, "jwt-token")
		assert.NoError(t, err)
	})

	t.Run("PreloadSkills with nil configs returns nil", func(t *testing.T) {
		err := preloader.PreloadSkills(ctx, 0, 0, nil, "")
		assert.NoError(t, err)
	})
}
