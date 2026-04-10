// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func ColumnExtractConfigPO2DO(po *model.ObservabilityColumnExtractConfig) *entity.ColumnExtractConfig {
	if po == nil {
		return nil
	}

	res := &entity.ColumnExtractConfig{
		ID:           po.ID,
		WorkspaceID:  po.WorkspaceID,
		PlatformType: po.PlatformType,
		SpanListType: po.SpanListType,
		AgentName:    po.AgentName,
		CreatedAt:    po.CreatedAt,
		CreatedBy:    po.CreatedBy,
		UpdatedAt:    po.UpdatedAt,
		UpdatedBy:    po.UpdatedBy,
	}

	if po.Config != nil && len(*po.Config) > 0 {
		var columns []entity.ColumnExtractRule
		if err := json.Unmarshal([]byte(*po.Config), &columns); err == nil {
			res.Columns = columns
		}
	}

	return res
}
