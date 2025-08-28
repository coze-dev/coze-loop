// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type ExptTurnAnnotateRecordRef struct {
	ID               int64
	SpaceID          int64
	ExptTurnResultID int64
	TagKeyID         int64
	AnnotateRecordID int64
	ExptID           int64
}

type ExptTurnResultTagRef struct {
	ID          int64
	SpaceID     int64
	ExptID      int64
	TagKeyID    int64
	TotalCnt    int32
	CompleteCnt int32
}

type AnnotateRecord struct {
	ID           int64
	SpaceID      int64
	TagKeyID     int64
	ExperimentID int64
	AnnotateData *AnnotateData
	BaseInfo     *BaseInfo
	TagValueID   int64
}

type AnnotateData struct {
	Score          *float64
	TextValue      *string
	BoolValue      *string // 标签基座boolean标签以选项形式实现
	Option         *string
	TagContentType TagContentType
}

type TagContentType string

const (
	TagContentTypeCategorical      = "categorical"
	TagContentTypeBoolean          = "boolean"
	TagContentTypeContinuousNumber = "continuous_number"
	TagContentTypeFreeText         = "free_text"

	TagContentTextMaxLength = 1024
)

type TagInfo struct {
	TagKeyId       int64
	TagKeyName     string
	Description    string
	InActive       bool
	TagContentType TagContentType
	TagValues      []*TagValue
	TagContentSpec *TagContentSpec
	TagStatus      TagStatus
}

type TagValue struct {
	TagValueId   int64
	TagValueName string
	Status       TagStatus // 选项是否被禁用
}

type TagContentSpec struct {
	ContinuousNumberSpec *ContinuousNumberSpec
}

type ContinuousNumberSpec struct {
	MinValue            *float64
	MinValueDescription *string
	MaxValue            *float64
	MaxValueDescription *string
}

type TagStatus = string

const (
	TagStatusActive     = "active"
	TagStatusInactive   = "inactive"
	TagStatusDeprecated = "deprecated"
)
