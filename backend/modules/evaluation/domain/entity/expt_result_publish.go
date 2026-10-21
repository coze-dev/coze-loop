// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import "context"

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

// ExptStandardEvalOutputSource freezes a Resulted canonical source for one standard output read.
// Snapshot is internal system data and must never be merged into target/evaluator input or content.
type ExptStandardEvalOutputSource struct {
	Snapshot *ExptItemResultPublishSource `json:"-"`
}

type exptStandardEvalOutputSourceKey struct{}

// WithExptStandardEvalOutputSource binds the existing standard result reader to this internal frozen source.
func WithExptStandardEvalOutputSource(ctx context.Context, source *ExptStandardEvalOutputSource) context.Context {
	return context.WithValue(ctx, exptStandardEvalOutputSourceKey{}, source)
}

func ExptStandardEvalOutputSourceFromContext(ctx context.Context) *ExptStandardEvalOutputSource {
	source, _ := ctx.Value(exptStandardEvalOutputSourceKey{}).(*ExptStandardEvalOutputSource)
	return source
}
