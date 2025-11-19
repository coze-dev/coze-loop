// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type PromptBasic struct {
	PromptType        PromptType `json:"prompt_type"`
	DisplayName       string     `json:"display_name"`
	Description       string     `json:"description"`
	LatestVersion     string     `json:"latest_version"`
	CreatedBy         string     `json:"created_by"`
	UpdatedBy         string     `json:"updated_by"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	LatestCommittedAt *time.Time `json:"latest_committed_at"`
}

type PromptType string

const (
	PromptTypeNormal  PromptType = "normal"
	PromptTypeSnippet PromptType = "snippet"
)
