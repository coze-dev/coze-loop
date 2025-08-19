// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExptEvalItem_SetState(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		item          *ExptEvalItem
		inputState    ItemRunState
		expectedState ItemRunState
		expectSameRef bool
	}{
		{
			name: "Set state to Queueing",
			item: &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Unknown,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Queueing,
			expectedState: ItemRunState_Queueing,
			expectSameRef: true,
		},
		{
			name: "Set state to Processing",
			item: &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Queueing,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Processing,
			expectedState: ItemRunState_Processing,
			expectSameRef: true,
		},
		{
			name: "Set state to Success",
			item: &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Processing,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Success,
			expectedState: ItemRunState_Success,
			expectSameRef: true,
		},
		{
			name: "Set state to Fail",
			item: &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Processing,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Fail,
			expectedState: ItemRunState_Fail,
			expectSameRef: true,
		},
		{
			name: "Set state to Terminal",
			item: &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Processing,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Terminal,
			expectedState: ItemRunState_Terminal,
			expectSameRef: true,
		},
		{
			name: "Set state to Unknown",
			item: &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Success,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Unknown,
			expectedState: ItemRunState_Unknown,
			expectSameRef: true,
		},
		{
			name: "Override Success state to Fail state",
			item: &ExptEvalItem{
				ExptID:           10,
				EvalSetVersionID: 20,
				ItemID:           30,
				State:            ItemRunState_Success,
				UpdatedAt:        &now,
			},
			inputState:    ItemRunState_Fail,
			expectedState: ItemRunState_Fail,
			expectSameRef: true,
		},
		{
			name:          "Set state for empty ExptEvalItem object",
			item:          &ExptEvalItem{},
			inputState:    ItemRunState_Processing,
			expectedState: ItemRunState_Processing,
			expectSameRef: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalExptID := tt.item.ExptID
			originalEvalSetVersionID := tt.item.EvalSetVersionID
			originalItemID := tt.item.ItemID
			originalUpdatedAt := tt.item.UpdatedAt

			result := tt.item.SetState(tt.inputState)

			assert.Equal(t, tt.expectedState, tt.item.State, "State should be set correctly")

			if tt.expectSameRef {
				assert.Same(t, tt.item, result, "Should return the same object reference for chain call support")
			}

			assert.Equal(t, originalExptID, tt.item.ExptID, "ExptID field should not be modified")
			assert.Equal(t, originalEvalSetVersionID, tt.item.EvalSetVersionID, "EvalSetVersionID field should not be modified")
			assert.Equal(t, originalItemID, tt.item.ItemID, "ItemID field should not be modified")
			assert.Equal(t, originalUpdatedAt, tt.item.UpdatedAt, "UpdatedAt field should not be modified")
		})
	}
}

func TestExptEvalItem_SetState_ChainCall(t *testing.T) {
	item := &ExptEvalItem{
		ExptID:           1,
		EvalSetVersionID: 2,
		ItemID:           3,
		State:            ItemRunState_Unknown,
	}

	result := item.SetState(ItemRunState_Queueing).SetState(ItemRunState_Processing).SetState(ItemRunState_Success)

	assert.Equal(t, ItemRunState_Success, item.State, "State should be Success after chain call")
	assert.Equal(t, ItemRunState_Success, result.State, "Returned object's state should be Success")
	assert.Same(t, item, result, "Chain call should return the same object")
}

func TestExptEvalItem_SetState_NilPointer(t *testing.T) {
	var item *ExptEvalItem

	assert.Panics(t, func() {
		item.SetState(ItemRunState_Processing)
	}, "Calling SetState on nil pointer should panic")
}

func TestExptEvalItem_SetState_AllStates(t *testing.T) {
	allStates := []ItemRunState{
		ItemRunState_Unknown,
		ItemRunState_Queueing,
		ItemRunState_Processing,
		ItemRunState_Success,
		ItemRunState_Fail,
		ItemRunState_Terminal,
	}

	for _, state := range allStates {
		t.Run(fmt.Sprintf("state_%d", int64(state)), func(t *testing.T) {
			item := &ExptEvalItem{
				ExptID:           1,
				EvalSetVersionID: 2,
				ItemID:           3,
				State:            ItemRunState_Unknown,
			}

			result := item.SetState(state)

			assert.Equal(t, state, item.State, "State should be set to %v", state)
			assert.Same(t, item, result, "Should return the same object reference")
		})
	}
}
