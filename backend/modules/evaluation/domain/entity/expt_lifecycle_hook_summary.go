// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

var ErrHookSummaryUnavailable = errors.New("hook summary unavailable")

const HookSummaryBatchLimit = 100

type LifecycleHookRunSummary struct {
	RunID         int64
	Before, After HookRunSummary
}

type HookRunSummary struct {
	Status      HookOperationStatus
	OperationID string
	Attempt     int32
	UpdatedAt   *time.Time
	Response    *spi.InvokeExperimentHookResponse
	Error       *spi.HookError
}

func NormalizeHookSummaryKeys(keys []HookRunKey) ([]HookRunKey, error) {
	if len(keys) > HookSummaryBatchLimit {
		return nil, invalidParam("hook summary batch exceeds limit")
	}
	seen := make(map[int64]HookRunKey, len(keys))
	out := make([]HookRunKey, 0, len(keys))
	for _, key := range keys {
		if key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID <= 0 {
			return nil, invalidParam("invalid hook summary run key")
		}
		if prior, exists := seen[key.RunID]; exists {
			if prior != key {
				return nil, invalidParam("conflicting hook summary run keys")
			}
			continue
		}
		seen[key.RunID] = key
		out = append(out, key)
	}
	return out, nil
}
