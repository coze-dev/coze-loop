// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// legacy 实验也必须回显 priority_level / scheduler_mode。
//
// 为什么不做"仅 enforce 才给"的裁剪：放量期最高频的疑问是"我这个实验为什么没进中心调度"，
// 那时最需要看的恰恰是一个 legacy 实验的 scheduler_mode —— 回显 legacy 就一眼确认没纳管，
// 不必去猜是 trigger 没带 evalx、灰度范围没命中还是代码没生效。
//
// 另一个理由是可分辨性：若只有 enforce 才给，调用方无法区分"这实验是 legacy"
// 与"这接口版本还不支持该字段"。
func TestToExptDTO_SchedulingReadView_Legacy(t *testing.T) {
	t.Parallel()

	dto := ToExptDTO(&entity.Experiment{
		ID:               1001,
		PriorityLevel:    1,
		ExptDispatchMode: entity.ExptDispatchModeLegacy,
		SchedulerScope:   "", // legacy 的 scope 恒为空
	})

	require.NotNil(t, dto)
	assert.Equal(t, int32(1), dto.GetPriorityLevel())
	assert.Equal(t, entity.ExptDispatchModeLegacy, dto.GetSchedulerMode())
	// legacy 没申报向量 → 省略而非返回空结构
	assert.Nil(t, dto.ExpectedQuotaConsumption)
}

func TestToExptDTO_SchedulingReadView_Enforce(t *testing.T) {
	t.Parallel()

	dto := ToExptDTO(&entity.Experiment{
		ID:               1002,
		PriorityLevel:    9,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
		SchedulerScope:   "fornax_cn_prod",
		EvalConf: &entity.EvaluationConfiguration{
			ExpectedQuotaConsumption: &entity.ExpectedQuotaConsumption{
				Resources: []*entity.ExpectedResourceConsumption{
					{Category: "sandbox", ResourceKey: "default", Amount: 1},
					{Category: "model", ResourceKey: "gpt5.5", Amount: 1000},
				},
			},
		},
	})

	require.NotNil(t, dto)
	assert.Equal(t, int32(9), dto.GetPriorityLevel())
	assert.Equal(t, entity.ExptDispatchModeEnforce, dto.GetSchedulerMode())

	require.NotNil(t, dto.ExpectedQuotaConsumption)
	res := dto.ExpectedQuotaConsumption.GetResources()
	require.Len(t, res, 2)
	assert.Equal(t, "sandbox", res[0].GetCategory())
	assert.Equal(t, "default", res[0].GetResourceKey())
	assert.Equal(t, int64(1), res[0].GetAmount())
	assert.Equal(t, "model", res[1].GetCategory())
	assert.Equal(t, "gpt5.5", res[1].GetResourceKey())
	assert.Equal(t, int64(1000), res[1].GetAmount())
}

// ★ scheduler_scope 一律不回显 —— 这是刻意的产品决定，不是遗漏。
//
// 它是不透明调度域 ID（形如 fornax_cn_prod），对调用方没有可用语义却泄露部署拓扑与
// 环境划分；业务代码本就不允许解析 Scope 字符串，回显只会诱使调用方依赖这个不稳定契约。
// 内部运维需要时直接查 experiment.scheduler_scope 列。
//
// 本测试用反射扫整个 DTO，确保**任何字段**都没把 scope 值带出去 ——
// 而不是只断言某个已知字段为空（那样将来有人新增字段仍会漏）。
func TestToExptDTO_SchedulerScopeNeverExposed(t *testing.T) {
	t.Parallel()

	const secretScope = "fornax_cn_ppe_fornax_evalx"
	dto := ToExptDTO(&entity.Experiment{
		ID:               1003,
		PriorityLevel:    5,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
		SchedulerScope:   secretScope,
	})
	require.NotNil(t, dto)

	assert.NotContains(t, dto.String(), secretScope,
		"scheduler_scope 不得出现在任何回显字段里；它只存在于 DB 列供内部运维查询")
}

// 历史/异常取值必须被收敛，读接口不该把脏数据原样吐出去。
func TestToExptDTO_SchedulingReadView_NormalizesDirtyValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		priority     int32
		mode         string
		wantPriority int32
		wantMode     string
	}{
		{"零值历史数据", 0, "", 1, entity.ExptDispatchModeLegacy},
		{"优先级越界偏低", -5, entity.ExptDispatchModeEnforce, 1, entity.ExptDispatchModeEnforce},
		{"优先级越界偏高", 100, entity.ExptDispatchModeEnforce, 99, entity.ExptDispatchModeEnforce},
		{"非法模式收敛为 legacy", 3, "bogus_mode", 3, entity.ExptDispatchModeLegacy},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dto := ToExptDTO(&entity.Experiment{
				ID:               2001,
				PriorityLevel:    tt.priority,
				ExptDispatchMode: tt.mode,
			})
			require.NotNil(t, dto)
			assert.Equal(t, tt.wantPriority, dto.GetPriorityLevel())
			assert.Equal(t, tt.wantMode, dto.GetSchedulerMode())
		})
	}
}

// 冻结向量为空结构时省略而非返回空 list：
// 让调用方能区分"没申报"与"申报了空向量"（后者是不应存在的数据异常）。
func TestToExptDTO_EmptyQuotaVectorOmitted(t *testing.T) {
	t.Parallel()

	dto := ToExptDTO(&entity.Experiment{
		ID: 3001,
		EvalConf: &entity.EvaluationConfiguration{
			ExpectedQuotaConsumption: &entity.ExpectedQuotaConsumption{Resources: nil},
		},
	})
	require.NotNil(t, dto)
	assert.Nil(t, dto.ExpectedQuotaConsumption)
}

// 向量里混入 nil 元素时跳过，不 panic 也不回显空壳资源。
func TestToExptDTO_QuotaVectorSkipsNilResource(t *testing.T) {
	t.Parallel()

	dto := ToExptDTO(&entity.Experiment{
		ID: 3002,
		EvalConf: &entity.EvaluationConfiguration{
			ExpectedQuotaConsumption: &entity.ExpectedQuotaConsumption{
				Resources: []*entity.ExpectedResourceConsumption{
					nil,
					{Category: "sandbox", ResourceKey: "default", Amount: 2},
				},
			},
		},
	})
	require.NotNil(t, dto)
	require.NotNil(t, dto.ExpectedQuotaConsumption)
	require.Len(t, dto.ExpectedQuotaConsumption.GetResources(), 1)
	assert.Equal(t, int64(2), dto.ExpectedQuotaConsumption.GetResources()[0].GetAmount())
}

// 全 nil 元素等价于没申报。
func TestToExptDTO_QuotaVectorAllNilOmitted(t *testing.T) {
	t.Parallel()

	dto := ToExptDTO(&entity.Experiment{
		ID: 3003,
		EvalConf: &entity.EvaluationConfiguration{
			ExpectedQuotaConsumption: &entity.ExpectedQuotaConsumption{
				Resources: []*entity.ExpectedResourceConsumption{nil, nil},
			},
		},
	})
	require.NotNil(t, dto)
	assert.Nil(t, dto.ExpectedQuotaConsumption)
}
