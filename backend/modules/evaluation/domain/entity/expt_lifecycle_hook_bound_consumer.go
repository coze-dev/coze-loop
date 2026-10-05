// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type HookBoundConsumerItem struct {
	SnapshotHash, PlanHash string
	PlanCount, RunVersion  int64
	Manifest               HookExecutionManifest
	ItemConfig             *ExptItemConfig
	Experiment             *Experiment
	Result                 *ExptItemEvalResult
}
