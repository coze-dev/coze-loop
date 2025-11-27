package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

func TrajectoryConfigPO2DO(po *model.ObservabilityTrajectoryConfig) *entity.TrajectoryConfig {
	if po == nil {
		return nil
	}
	return &entity.TrajectoryConfig{
		ID:          po.ID,
		WorkspaceID: po.WorkspaceID,
		Filter:      po.Filter,
		CreatedAt:   po.CreatedAt,
		CreatedBy:   po.CreatedBy,
		UpdatedAt:   po.UpdatedAt,
		UpdatedBy:   po.UpdatedBy,
	}
}
