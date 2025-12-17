// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TrajectoryConfigPO2DO(po *model.ObservabilityTrajectoryConfig) *entity.TrajectoryConfig {
	if po == nil {
		return nil
	}

	res := &entity.TrajectoryConfig{
		ID:          po.ID,
		WorkspaceID: po.WorkspaceID,
		CreatedAt:   po.CreatedAt,
		CreatedBy:   po.CreatedBy,
		UpdatedAt:   po.UpdatedAt,
		UpdatedBy:   po.UpdatedBy,
	}

	if po.Filter != nil && len(*po.Filter) > 0 {
		filters := &loop_span.FilterFields{}
		if err := json.Unmarshal([]byte(*po.Filter), &filters); err == nil {
			res.Filter = filters
		}
	}

	return res
}
