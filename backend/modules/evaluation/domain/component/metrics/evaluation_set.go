// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import "context"

//go:generate mockgen -destination=mocks/evaluation_set.go -package=mocks . EvaluationSetMetrics
type EvaluationSetMetrics interface {
	EmitCreate(spaceID int64, err error)
}

// OpenAPIEvaluationSetMetrics OpenAPI专用的评测集指标接口
//go:generate mockgen -destination=mocks/openapi_evaluation_set.go -package=mocks . OpenAPIEvaluationSetMetrics
type OpenAPIEvaluationSetMetrics interface {
	EmitOpenAPIMetric(ctx context.Context, spaceID, evaluationSetID int64, method string, success bool)
}