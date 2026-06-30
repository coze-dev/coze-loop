// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// EvaluatorCallbackPayload 异步评估器执行完成回调的 POST body
type EvaluatorCallbackPayload struct {
	DeliveryID         string               `json:"delivery_id"`
	InvokeID           int64                `json:"invoke_id"`
	WorkspaceID        int64                `json:"workspace_id"`
	EvaluatorVersionID int64                `json:"evaluator_version_id"`
	Status             string               `json:"status"` // success | fail
	Output             *EvaluatorOutputData `json:"output,omitempty"`
	TimeConsumingMS    int64                `json:"time_consuming_ms"`
}
