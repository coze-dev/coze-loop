package entity

import "time"

type PromptLabel struct {
	ID        int64     `json:"id"`
	SpaceID   int64     `json:"space_id"`
	LabelKey  string    `json:"label_key"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}
