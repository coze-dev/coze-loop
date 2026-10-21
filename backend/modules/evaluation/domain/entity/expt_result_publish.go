// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

// ExptItemResultPublishSource is a single primary database observation, frozen
// before record reads and publication. It is never sent to a target/evaluator.
type ExptItemResultPublishSource struct {
	Source         *ExptCurrentItemRunSource
	Item           *ExptItemResult
	ItemLog        *ExptItemResultRunLog
	ItemRef        *ExptItemRef
	ItemConfigJSON []byte
}

// ExptItemResultPublication contains only the projection prepared from Source.
// IDs and scores are prepared before entering the repository transaction.
type ExptItemResultPublication struct {
	Source *ExptItemResultPublishSource
	Turns  []*ExptTurnResult
	Refs   []*ExptTurnEvaluatorResultRef
}
