// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// callTarget 里的两处 Ext 透传是**多轮/SUA 配置进入执行期的唯一出口**:
//
//	ext["builtin_run_conf"]        ← ItemConfig.EvalTargetConf.RunConf   (题目级)
//	ext["builtin_run_mode_config"] ← Expt.EvalConf.RunModeConfig         (实验级)
//
// 两处都是 `if 非 nil { 写 }` 形态, 所以回归形状很固定: 条件写错 / key 写错 / 序列化对象取错,
// 全都表现为算子那侧读到空串 → 按缺省跑 → 实验 success 而配置没生效。
// 这条链在本分支之前的整段历史上零覆盖 (既有 callTarget 用例全部不带 ItemConfig / RunModeConfig)。

// 组一份最小可执行的 callTarget 上下文: 一个题目 (turn 里带一个字段) + 一个 target。
func newRunConfEtec(itemConfig *entity.ExptItemConfig, runModeConfig *entity.RunModeConfig) *entity.ExptTurnEvalCtx {
	content := &entity.Content{Text: gptr.Of("hello")}
	return &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event:       &entity.ExptItemEvalEvent{ExptRunID: 1},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
			ItemConfig:  itemConfig,
			Expt: &entity.Experiment{
				Target: &entity.EvalTarget{ID: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1}},
				EvalConf: &entity.EvaluationConfiguration{
					RunModeConfig: runModeConfig,
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 1,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
								},
							},
						},
					},
				},
			},
		},
		Turn: &entity.Turn{ID: 1, FieldDataList: []*entity.FieldData{{Name: "field1", Content: content}}},
		Ext:  map[string]string{},
	}
}

// 题目级 + 实验级两份配置都必须原样落到 Ext, 且**各自落在自己的 key 上**。
//
// 两个 key 名字只差一个词、在同一个函数里前后相邻写入, 是最容易被复制粘贴写错的一对;
// 写串了的现象是"题目级配置被当成实验级 (或反之)", 而两者的合并语义完全不同
// (题目级优先、实验级兜底), 于是单题特例静默失效或反过来污染整个实验。
func TestCallTarget_PassesRunConfAndRunModeConfigToExt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	svc := &DefaultExptTurnEvaluationImpl{metric: mockMetric, evalTargetService: mockTargetSvc}

	itemConfig := &entity.ExptItemConfig{EvalTargetConf: &entity.ItemTargetConf{
		TargetVersionID: 1,
		RunConf: &entity.ItemRunConf{
			MaxTurns:         3,
			EvaluatorTrigger: "final_turn_only",
			FixedQueryList: []*entity.FixedQuery{{
				ComplexQuery: &entity.ComplexQuery{Parts: []*entity.ContentPart{
					{ContentType: "text", Text: "第一轮"},
				}},
			}},
		},
	}}
	runModeConfig := &entity.RunModeConfig{
		RunMode:    entity.RunModeFixedScriptMultiTurn,
		SuaMode:    entity.SuaModeLoop,
		MaxTurns:   16,
		SuaPersona: "资深工程师",
	}

	mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
	mockTargetSvc.EXPECT().ExecuteTarget(gomock.Any(), int64(123), int64(1), int64(1), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, in *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
			// --- 题目级 ---
			rawItem, ok := in.Ext[consts.TargetExecuteExtRunConfKey]
			require.Truef(t, ok, "ext[%s] 缺失 —— 题目级多轮配置到不了算子, 每题都会按实验级默认值跑",
				consts.TargetExecuteExtRunConfKey)
			var gotItem entity.ItemRunConf
			require.NoError(t, json.Unmarshal([]byte(rawItem), &gotItem))
			assert.Equal(t, 3, gotItem.MaxTurns)
			assert.Equal(t, "final_turn_only", gotItem.EvaluatorTrigger)
			// fixed_query_list 里的 complex_query 必须活着到这一跳 —— 它丢了就是那次 16 轮空跑。
			require.Len(t, gotItem.FixedQueryList, 1)
			require.NotNil(t, gotItem.FixedQueryList[0].ComplexQuery)
			assert.Equal(t, "第一轮", gotItem.FixedQueryList[0].ComplexQuery.Parts[0].Text)

			// --- 实验级 ---
			rawExpt, ok := in.Ext[consts.TargetExecuteExtRunModeConfigKey]
			require.Truef(t, ok, "ext[%s] 缺失 —— 整个实验会回落默认跑法",
				consts.TargetExecuteExtRunModeConfigKey)
			var gotExpt entity.RunModeConfig
			require.NoError(t, json.Unmarshal([]byte(rawExpt), &gotExpt))
			assert.Equal(t, entity.RunModeFixedScriptMultiTurn, gotExpt.RunMode)
			assert.Equal(t, entity.SuaModeLoop, gotExpt.SuaMode)
			assert.Equal(t, 16, gotExpt.MaxTurns)
			assert.Equal(t, "资深工程师", gotExpt.SuaPersona)

			// 两个 key 不能写串: 题目级那份不该出现实验级独有的字段, 反之亦然。
			assert.NotEqual(t, rawItem, rawExpt, "两份配置写成了同一个内容 —— 说明序列化对象取错了")
			assert.NotContains(t, rawItem, `"run_mode"`, "题目级 ext 里不该有 run_mode (那是实验级字段)")
			return &entity.EvalTargetRecord{ID: 1}, nil
		})

	record, err := svc.callTarget(context.Background(), newRunConfEtec(itemConfig, runModeConfig), nil, 123)
	require.NoError(t, err)
	assert.Equal(t, int64(1), record.ID)
}

// 未配多轮的实验 (存量实验就是这样) 两个 key **都不能出现**。
//
// 这不是洁癖: 消费侧判"未配"的依据是 `raw == ""`(key 不存在)。凭空写一个 "{}" 或 "null"
// 进去, 消费侧就认为"配过了", 于是「Ext 为空则回退读题目自带 run_conf」这条回退永远走不到 ——
// 评测集题目里自带的多轮配置一个都到不了 runtime, 且全程零报错。这正是本分支修过的一个真 bug。
func TestCallTarget_OmitsRunConfKeysWhenUnset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cases := []struct {
		name       string
		itemConfig *entity.ExptItemConfig
		runMode    *entity.RunModeConfig
	}{
		{
			// 老 DataSet 实验: 压根没有 ItemConfig。
			name: "ItemConfig 为 nil (老 DataSet 实验)",
		},
		{
			// MultiSetConfig 但非沙箱对象: ItemConfig 有、RunConf 为 nil。
			name:       "ItemConfig 在但 RunConf 为 nil",
			itemConfig: &entity.ExptItemConfig{EvalTargetConf: &entity.ItemTargetConf{TargetVersionID: 1}},
		},
		{
			// ItemConfig 存在但 EvalTargetConf 缺失 —— 防的是取字段时漏判空导致 panic。
			name:       "ItemConfig 在但 EvalTargetConf 为 nil",
			itemConfig: &entity.ExptItemConfig{},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			mockMetric := metricsmocks.NewMockExptMetric(ctrl)
			mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
			svc := &DefaultExptTurnEvaluationImpl{metric: mockMetric, evalTargetService: mockTargetSvc}

			mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
			mockTargetSvc.EXPECT().ExecuteTarget(gomock.Any(), int64(123), int64(1), int64(1), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, in *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					assert.NotContains(t, in.Ext, consts.TargetExecuteExtRunConfKey,
						`未配题目级 run_conf 时不能凭空写 key —— 消费侧会因此认为"配过了", 从而不再回退读题目自带配置`)
					assert.NotContains(t, in.Ext, consts.TargetExecuteExtRunModeConfigKey,
						"未配实验级跑法时不能凭空写 key")
					return &entity.EvalTargetRecord{ID: 1}, nil
				})

			_, err := svc.callTarget(context.Background(), newRunConfEtec(c.itemConfig, c.runMode), nil, 123)
			require.NoError(t, err)
		})
	}
}

// 两级配置**互相独立**: 只配一级时, 另一级的 key 不能被顺带写上。
//
// 合并规则是"题目级优先、实验级兜底", 而合并发生在 runtime 侧 —— 平台只如实下发两级值。
// 若平台在这里替对端做了预合并(把实验级值抄进题目级那份), 实验级值就会伪装成题目级下发,
// runtime 再也分不出"这个值来自哪一级", 题目优先语义直接失效。所以两级必须严格分离。
func TestCallTarget_TwoLevelConfigsAreIndependent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("只有实验级", func(t *testing.T) {
		mockMetric := metricsmocks.NewMockExptMetric(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		svc := &DefaultExptTurnEvaluationImpl{metric: mockMetric, evalTargetService: mockTargetSvc}

		mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
		mockTargetSvc.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, in *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				assert.Contains(t, in.Ext, consts.TargetExecuteExtRunModeConfigKey)
				assert.NotContains(t, in.Ext, consts.TargetExecuteExtRunConfKey,
					"实验级值不能被抄成题目级下发 —— 那样 runtime 就分不出值来自哪一级, 题目优先失效")
				return &entity.EvalTargetRecord{ID: 1}, nil
			})

		_, err := svc.callTarget(context.Background(),
			newRunConfEtec(nil, &entity.RunModeConfig{RunMode: entity.RunModeSUAMultiTurn, SuaGoal: "expt-level"}), nil, 123)
		require.NoError(t, err)
	})

	t.Run("只有题目级", func(t *testing.T) {
		mockMetric := metricsmocks.NewMockExptMetric(ctrl)
		mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
		svc := &DefaultExptTurnEvaluationImpl{metric: mockMetric, evalTargetService: mockTargetSvc}

		mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
		mockTargetSvc.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, in *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				assert.Contains(t, in.Ext, consts.TargetExecuteExtRunConfKey)
				assert.NotContains(t, in.Ext, consts.TargetExecuteExtRunModeConfigKey,
					"题目级配置不该让实验级 key 凭空出现")
				return &entity.EvalTargetRecord{ID: 1}, nil
			})

		itemConfig := &entity.ExptItemConfig{EvalTargetConf: &entity.ItemTargetConf{
			TargetVersionID: 1,
			RunConf:         &entity.ItemRunConf{SuaGoal: "item-level"},
		}}
		_, err := svc.callTarget(context.Background(), newRunConfEtec(itemConfig, nil), nil, 123)
		require.NoError(t, err)
	})
}

// 透传不得破坏 etec.Ext 里已有的键 (它承载 runtime_param 等既有透传项), 也不得回写调用方的 map。
//
// callTarget 对 Ext 是 clone 后再写 —— 若退化成直接写原 map, 同一个 etec 被复用时
// (重试链路会复用 ctx) 上一次的多轮配置会残留到下一次, 表现为"改了配置但跑的还是旧的"。
func TestCallTarget_ExtPassthroughDoesNotMutateCallerMap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	svc := &DefaultExptTurnEvaluationImpl{metric: mockMetric, evalTargetService: mockTargetSvc}

	etec := newRunConfEtec(
		&entity.ExptItemConfig{EvalTargetConf: &entity.ItemTargetConf{TargetVersionID: 1, RunConf: &entity.ItemRunConf{MaxTurns: 2}}},
		&entity.RunModeConfig{RunMode: entity.RunModeSingleTurn},
	)
	etec.Ext = map[string]string{"pre_existing": "keep-me"}

	mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
	mockTargetSvc.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, in *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
			assert.Equal(t, "keep-me", in.Ext["pre_existing"], "既有 Ext 项不能被多轮透传挤掉")
			assert.Contains(t, in.Ext, consts.TargetExecuteExtRunConfKey)
			return &entity.EvalTargetRecord{ID: 1}, nil
		})

	_, err := svc.callTarget(context.Background(), etec, nil, 123)
	require.NoError(t, err)

	// 调用方那份 map 必须没被污染 —— 否则重试复用 ctx 时会带上一轮的残留配置。
	assert.Len(t, etec.Ext, 1, "callTarget 不该往调用方的 Ext 里回写, 否则重试会带上旧配置残留")
	assert.NotContains(t, etec.Ext, consts.TargetExecuteExtRunConfKey)
	assert.NotContains(t, etec.Ext, consts.TargetExecuteExtRunModeConfigKey)
}
