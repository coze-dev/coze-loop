// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type ResourceTagRef struct {
	TagName string `json:"tag_name"`
}

type ResourceTag struct {
	TagName     string `json:"tag_name"`
	TagKeyID    int64  `json:"tag_key_id,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Status      string `json:"status,omitempty"`
}

type TagFilterRelation string

const (
	TagFilterRelationOr  TagFilterRelation = "or"
	TagFilterRelationAnd TagFilterRelation = "and"
)

type TagFilter struct {
	TagNames []string          `json:"tag_names"`
	Relation TagFilterRelation `json:"relation,omitempty"`
}
