// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
)

func TestEvalTargetVersionDO2PO(t *testing.T) {
	now := time.Now()
	nowMilli := now.UnixMilli()
	userID := "test-user"

	validCozeBot := &entity.CozeBot{BotID: 123, BotVersion: "v1"}
	validPrompt := &entity.LoopPrompt{PromptID: 456, Version: "v2"}
	validWorkflow := &entity.CozeWorkflow{ID: "789", Version: "v3"}
	validInputSchema := []*entity.ArgsSchema{{Key: gptr.Of("input")}}
	validOutputSchema := []*entity.ArgsSchema{{Key: gptr.Of("output")}}

	cozeBotJSON, _ := json.Marshal(validCozeBot)
	promptJSON, _ := json.Marshal(validPrompt)
	workflowJSON, _ := json.Marshal(validWorkflow)
	inputSchemaJSON, _ := json.Marshal(validInputSchema)
	outputSchemaJSON, _ := json.Marshal(validOutputSchema)

	tests := []struct {
		name    string
		do      *entity.EvalTargetVersion
		wantPO  *model.TargetVersion
		wantErr bool
	}{
		{
			name: "success - CozeBot type",
			do: &entity.EvalTargetVersion{
				ID:                  1,
				SpaceID:             10,
				TargetID:            100,
				SourceTargetVersion: "v1.0",
				EvalTargetType:      entity.EvalTargetTypeCozeBot,
				CozeBot:             validCozeBot,
				InputSchema:         validInputSchema,
				OutputSchema:        validOutputSchema,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of(userID)},
					UpdatedBy: &entity.UserInfo{UserID: gptr.Of(userID)},
					CreatedAt: gptr.Of(nowMilli),
					UpdatedAt: gptr.Of(nowMilli),
				},
			},
			wantPO: &model.TargetVersion{
				ID:                  1,
				SpaceID:             10,
				TargetID:            100,
				SourceTargetVersion: "v1.0",
				TargetMeta:          &cozeBotJSON,
				InputSchema:         &inputSchemaJSON,
				OutputSchema:        &outputSchemaJSON,
				CreatedBy:           userID,
				UpdatedBy:           userID,
				CreatedAt:           time.UnixMilli(nowMilli),
				UpdatedAt:           time.UnixMilli(nowMilli),
			},
			wantErr: false,
		},
		{
			name: "success - LoopPrompt type",
			do: &entity.EvalTargetVersion{
				ID:                  2,
				SpaceID:             20,
				TargetID:            200,
				SourceTargetVersion: "v2.0",
				EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
				Prompt:              validPrompt,
				InputSchema:         validInputSchema,
				OutputSchema:        validOutputSchema,
			},
			wantPO: &model.TargetVersion{
				ID:                  2,
				SpaceID:             20,
				TargetID:            200,
				SourceTargetVersion: "v2.0",
				TargetMeta:          &promptJSON,
				InputSchema:         &inputSchemaJSON,
				OutputSchema:        &outputSchemaJSON,
			},
			wantErr: false,
		},
		{
			name: "success - CozeWorkflow type",
			do: &entity.EvalTargetVersion{
				ID:                  3,
				SpaceID:             30,
				TargetID:            300,
				SourceTargetVersion: "v3.0",
				EvalTargetType:      entity.EvalTargetTypeCozeWorkflow,
				CozeWorkflow:        validWorkflow,
				InputSchema:         validInputSchema,
				OutputSchema:        validOutputSchema,
			},
			wantPO: &model.TargetVersion{
				ID:                  3,
				SpaceID:             30,
				TargetID:            300,
				SourceTargetVersion: "v3.0",
				TargetMeta:          &workflowJSON,
				InputSchema:         &inputSchemaJSON,
				OutputSchema:        &outputSchemaJSON,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			po, err := EvalTargetVersionDO2PO(tt.do)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, po)
			} else {
				assert.NoError(t, err)
				// Since json marshal for nil slice is "null", we handle it specifically
				if tt.do.InputSchema == nil {
					var nullBytes []byte
					nullBytes, _ = json.Marshal(nil)
					tt.wantPO.InputSchema = &nullBytes
				}
				if tt.do.OutputSchema == nil {
					var nullBytes []byte
					nullBytes, _ = json.Marshal(nil)
					tt.wantPO.OutputSchema = &nullBytes
				}

				assert.Equal(t, tt.wantPO.ID, po.ID)
				assert.Equal(t, tt.wantPO.SpaceID, po.SpaceID)
				assert.Equal(t, tt.wantPO.TargetID, po.TargetID)
				assert.Equal(t, tt.wantPO.SourceTargetVersion, po.SourceTargetVersion)
				assert.Equal(t, tt.wantPO.CreatedBy, po.CreatedBy)
				assert.Equal(t, tt.wantPO.UpdatedBy, po.UpdatedBy)
				assert.WithinDuration(t, tt.wantPO.CreatedAt, po.CreatedAt, time.Millisecond)
				assert.WithinDuration(t, tt.wantPO.UpdatedAt, po.UpdatedAt, time.Millisecond)

				if tt.wantPO.TargetMeta != nil {
					assert.JSONEq(t, string(*tt.wantPO.TargetMeta), string(*po.TargetMeta))
				} else {
					assert.Nil(t, po.TargetMeta)
				}
				if tt.wantPO.InputSchema != nil {
					assert.JSONEq(t, string(*tt.wantPO.InputSchema), string(*po.InputSchema))
				} else {
					assert.Nil(t, po.InputSchema)
				}
				if tt.wantPO.OutputSchema != nil {
					assert.JSONEq(t, string(*tt.wantPO.OutputSchema), string(*po.OutputSchema))
				} else {
					assert.Nil(t, po.OutputSchema)
				}
			}
		})
	}
}

func TestEvalTargetPO2DOs(t *testing.T) {
	// a test case for EvalTargetPO2DOs
	pos := []*model.Target{
		{
			ID:             1,
			SpaceID:        1,
			SourceTargetID: "1",
			TargetType:     1,
		},
	}
	dos := EvalTargetPO2DOs(pos)
	assert.Equal(t, len(pos), len(dos))
	assert.Equal(t, pos[0].ID, dos[0].ID)
}
