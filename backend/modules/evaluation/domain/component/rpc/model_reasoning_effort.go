// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

type IModelReasoningEffortProvider interface {
	GetReasoningEffortOptions(ctx context.Context, spaceID, modelID int64) (bool, []string, error)
}
