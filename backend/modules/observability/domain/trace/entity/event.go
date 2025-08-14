// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"

type AnnotationEvent struct {
	Annotation *loop_span.Annotation `json:"annotation"`
	StartAt    int64                 `json:"start_at"` // ms
	EndAt      int64                 `json:"end_at"`   // ms
	Caller     string                `json:"caller"`
	RetryTimes int64                 `json:"retry_times"`
}
