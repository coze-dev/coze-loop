package entity

import "time"

type TrajectoryConfig struct {
	ID          int64
	WorkspaceID int64
	Filter      *string
	CreatedAt   time.Time
	CreatedBy   string
	UpdatedAt   time.Time
	UpdatedBy   string
}
