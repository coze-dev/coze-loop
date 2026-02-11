// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
)

func TestEvalTargetRecordConvert(t *testing.T) {
	t.Run("DO2PO", func(t *testing.T) {
		do := &entity.EvalTargetRecord{
			ID:      1,
			SpaceID: 2,
			Status:  gptr.Of(entity.EvalTargetRunStatusSuccess),
			EvalTargetInputData: &entity.EvalTargetInputData{
				InputFields: map[string]*entity.Content{"k": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("v")}},
			},
			EvalTargetOutputData: &entity.EvalTargetOutputData{
				OutputFields: map[string]*entity.Content{"res": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("resp")}},
			},
			BaseInfo: &entity.BaseInfo{
				CreatedAt: gptr.Of(int64(123456789)),
			},
		}
		po, err := EvalTargetRecordDO2PO(do)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), po.ID)
		assert.Equal(t, int32(entity.EvalTargetRunStatusSuccess), po.Status)
		assert.NotNil(t, po.InputData)
		assert.NotNil(t, po.OutputData)

		poNil, errNil := EvalTargetRecordDO2PO(nil)
		assert.NoError(t, errNil)
		assert.Nil(t, poNil)
	})

	t.Run("PO2DO", func(t *testing.T) {
		input := []byte(`{"InputFields":{"k":{"text":"v"}}}`)
		output := []byte(`{"OutputFields":{"res":{"text":"resp"}},"EvalTargetUsage":{"InputTokens":10,"OutputTokens":20}}`)
		po := &model.TargetRecord{
			ID:         1,
			Status:     int32(entity.EvalTargetRunStatusSuccess),
			InputData:  &input,
			OutputData: &output,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		do, err := EvalTargetRecordPO2DO(po)
		assert.NoError(t, err)
		assert.NotNil(t, do)
		assert.NotNil(t, do.EvalTargetInputData)
		assert.NotNil(t, do.EvalTargetInputData.InputFields["k"])
		assert.Equal(t, int64(1), do.ID)
		assert.Equal(t, entity.EvalTargetRunStatusSuccess, *do.Status)
		assert.Equal(t, "v", *do.EvalTargetInputData.InputFields["k"].Text)
		assert.Equal(t, int64(30), do.EvalTargetOutputData.EvalTargetUsage.TotalTokens)

		doNil, errNilPo := EvalTargetRecordPO2DO(nil)
		assert.NoError(t, errNilPo)
		assert.Nil(t, doNil)
	})

	t.Run("PO2DO_unmarshal_error", func(t *testing.T) {
		input := []byte(`{invalid}`)
		po := &model.TargetRecord{InputData: &input}
		_, err := EvalTargetRecordPO2DO(po)
		assert.Error(t, err)
	})
}
