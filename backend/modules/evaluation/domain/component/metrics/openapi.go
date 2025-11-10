// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import "context"

//go:generate mockgen -destination=mocks/openapi_evaluation_set.go -package=mocks . OpenAPIEvaluationMetrics
type OpenAPIEvaluationMetrics interface {
	EmitOpenAPIMetric(ctx context.Context, spaceID, evaluationSetID int64, method string, startTime int64, err error)
}
