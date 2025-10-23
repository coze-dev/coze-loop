// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

//go:generate mockgen -destination=mocks/evaluation_set.go -package=mocks . EvaluationSetMetrics
type EvaluationSetMetrics interface {
	EmitCreate(spaceID int64, err error)
}
