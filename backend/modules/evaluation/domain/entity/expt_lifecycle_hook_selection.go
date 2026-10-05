// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type HookSelectionInput struct {
	Key                HookRunKey
	Mode               ExptRunMode
	Experiment         *Experiment
	HasExplicitItemIDs bool
	ItemIDs            []int64
	Cursor             string
}

type HookSelectionPage struct {
	// Candidates have ID=0; the plan writer allocates storage IDs.
	Items []HookPlanItem
	// Persist NextCursor even when Done; resuming it returns Done without rereading the source.
	NextCursor string
	Done       bool
}
