// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type TagFilterRelation string

const (
	TagFilterRelationAND TagFilterRelation = "and"
	TagFilterRelationOR  TagFilterRelation = "or"
)

type ResourceTagRef struct {
	TagKeyID   int64
	TagValueID *int64
}

type ResourceTag struct {
	TagKeyID     int64
	TagKeyName   string
	TagValueID   *int64
	TagValueName string
	ContentType  string
	Status       string
}

type TagFilter struct {
	TagKeyIDs []int64
	Tags      []*ResourceTagRef
	Relation  *TagFilterRelation
}
