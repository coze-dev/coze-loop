// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// TestExptItemResultRunLogConverter_RetryTimes 覆盖 §6.6:
// PO2DO / DO2PO 双向搬运 retry_times; 旧数据(列默认 0)读出为 0(旧数据默认视为未重试)。
func TestExptItemResultRunLogConverter_RetryTimes(t *testing.T) {
	c := NewExptItemResultRunLogConverter()

	t.Run("DO2PO carries retry_times", func(t *testing.T) {
		do := &entity.ExptItemResultRunLog{ID: 1, SpaceID: 2, ItemID: 3, RetryTimes: 4}
		po := c.DO2PO(do)
		assert.Equal(t, int32(4), po.RetryTimes)
	})

	t.Run("PO2DO carries retry_times", func(t *testing.T) {
		po := &model.ExptItemResultRunLog{ID: 1, SpaceID: 2, ItemID: 3, RetryTimes: 7}
		do := c.PO2DO(po)
		assert.Equal(t, int32(7), do.RetryTimes)
	})

	t.Run("roundtrip DO->PO->DO preserves retry_times", func(t *testing.T) {
		do := &entity.ExptItemResultRunLog{ID: 1, SpaceID: 2, ItemID: 3, RetryTimes: 9}
		got := c.PO2DO(c.DO2PO(do))
		assert.Equal(t, int32(9), got.RetryTimes)
	})

	t.Run("legacy row (retry_times zero-value) reads as 0 = never retried", func(t *testing.T) {
		po := &model.ExptItemResultRunLog{ID: 1, SpaceID: 2, ItemID: 3} // RetryTimes 未设置, 零值
		do := c.PO2DO(po)
		assert.Equal(t, int32(0), do.RetryTimes)
	})

	t.Run("new-build zero-value DO maps to 0 (not dropped/non-zero)", func(t *testing.T) {
		// 对应 design §3.6: 5 处重跑新建行结构体不设 RetryTimes, DO2PO 必须映射成 0
		do := &entity.ExptItemResultRunLog{ID: 1, SpaceID: 2, ItemID: 3}
		po := c.DO2PO(do)
		assert.Equal(t, int32(0), po.RetryTimes)
	})
}
