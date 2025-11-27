package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type TrajectoryConfig struct {
	ID          int64
	WorkspaceID int64
	Filter      *loop_span.FilterFields
	CreatedAt   time.Time
	CreatedBy   string
	UpdatedAt   time.Time
	UpdatedBy   string
}
