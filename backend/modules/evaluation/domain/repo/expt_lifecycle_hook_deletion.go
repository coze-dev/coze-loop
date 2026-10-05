// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Deletion and all original-Run terminal intents commit together. Only newly deleted rows are returned.
type IHookDeletionRepo interface {
	DeleteExperiments(ctx context.Context, ids []int64, workspaceID int64, executionScope string) ([]*entity.Experiment, error)
}
