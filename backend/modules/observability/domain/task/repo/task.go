package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
)

//go:generate mockgen -destination=mocks/Task.go -package=mocks . ITaskRepo
type ITaskRepo interface {
	GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityTask, error)
	ListTasks(ctx context.Context, workspaceID int64, userID string) ([]*entity.ObservabilityTask, error)
	UpdateTask(ctx context.Context, do *entity.ObservabilityTask) error
	CreateTask(ctx context.Context, do *entity.ObservabilityTask) (int64, error)
	DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error
}
