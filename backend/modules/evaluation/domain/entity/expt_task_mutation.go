// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "context"

type taskExperimentMutationKey struct{}

// WithTaskExperimentMutation is for task adapters after ownership and current-source validation.
func WithTaskExperimentMutation(ctx context.Context) context.Context {
	return context.WithValue(ctx, taskExperimentMutationKey{}, true)
}

func IsTaskExperimentMutation(ctx context.Context) bool {
	allowed, _ := ctx.Value(taskExperimentMutationKey{}).(bool)
	return allowed
}

func ValidateMaxRunMinutes(minutes int) bool {
	return minutes > 0
}

// IsNeverStartedQueuedItem also requires the caller to verify a complete ref and an unreserved dispatch observation.
func IsNeverStartedQueuedItem(item *ExptItemResult, log *ExptItemResultRunLog) bool {
	if item == nil || log == nil || item.SpaceID <= 0 || item.ExptID <= 0 || item.ExptRunID <= 0 || item.ItemID <= 0 {
		return false
	}
	if item.SpaceID != log.SpaceID || item.ExptID != log.ExptID || item.ExptRunID != log.ExptRunID || item.ItemID != log.ItemID || item.ItemVersionID != log.ItemVersionID {
		return false
	}
	_, manualRetry := item.Ext[ManualRetryExtKey]
	return item.Status == ItemRunState_Queueing && log.Status == int32(ItemRunState_Queueing) &&
		item.LogID == "" && log.LogID == "" && item.ErrMsg == "" && len(log.ErrMsg) == 0 &&
		log.RetryTimes == 0 && log.ResultState == int32(ExptItemResultStateDefault) &&
		log.QuotaReservationState == QuotaReservationStateNone && !manualRetry
}

type RetryItemsRunReceipt struct {
	RunID           int64
	AcceptedItemIDs []int64
	RejectedItems   []*ManualRetryRejectedItem
	SnapshotAtMS    int64
}
