// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	metricsmock "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// 模板化建集把 CreateEvaluationSet 的入参契约从"schema 必填"改成了"schema 或 template_dataset_id
// 二者至少其一"。这个放宽是必要的(用模板建集时列由模板决定, 调用方不必传 schema), 但它同时
// 打开了两个新的错误入口, 且两者都**不会在下游被兜住**:
//
//  1. 两个都不传 → 建出一个零列评测集并返回成功。用户要到往里导数据时才发现字段全对不上。
//  2. template_dataset_id <= 0 → 那不是任何真实模板的 id。0 是 int64 零值, 前端漏填/序列化
//     丢字段都会得到 0 —— 它会被当成"传了模板"一路往下走, 到模板服务那里查不到,
//     报出来的错跟"没填模板"毫无关联性(商业版实现甚至可能把 0 当成"取默认模板")。
//
// 两条都必须在 handler 入口拒, 所以这里逐条钉住。

func newCreateEvalSetApp(ctrl *gomock.Controller) (*EvaluationSetApplicationImpl, *rpcmocks.MockIAuthProvider, *servicemocks.MockIEvaluationSetService) {
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	svc := servicemocks.NewMockIEvaluationSetService(ctrl)
	metric := metricsmock.NewMockEvaluationSetMetrics(ctrl)
	// EmitCreate 在 defer 里无条件调用(含参数校验失败的早退路径)。
	metric.EXPECT().EmitCreate(gomock.Any(), gomock.Any()).AnyTimes()
	return &EvaluationSetApplicationImpl{auth: auth, evaluationSetService: svc, metric: metric}, auth, svc
}

// 参数校验必须在**鉴权之前**完成, 且非法请求一律不落到 domain 层。
//
// 断言"domain 一次都没被调到"是这组用例的核心: 只断言 err != nil 不够 —— 校验被误删时
// 请求会带着零值参数继续往下走, 也可能因为下游报错而返回 error, 测试照样绿, 而实际上
// 已经建出了一个坏评测集 (或对模板服务发了一次 id=0 的查询)。
func TestEvaluationSetApplicationImpl_CreateEvaluationSet_TemplateParamValidation(t *testing.T) {
	t.Run("schema 与 template_dataset_id 都不传 → 拒绝", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, auth, svc := newCreateEvalSetApp(ctrl)
		// 校验在鉴权之前: 鉴权与 domain 都不该被调到。
		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
		svc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)

		_, err := app.CreateEvaluationSet(context.Background(), &eval_set.CreateEvaluationSetRequest{
			WorkspaceID: 1,
			Name:        gptr.Of("no-schema-no-template"),
		})
		require.Error(t, err, "两者都不传会建出零列评测集并返回成功 —— 必须入口拒")
	})

	t.Run("template_dataset_id=0 → 拒绝", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, auth, svc := newCreateEvalSetApp(ctrl)
		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
		svc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)

		// 0 是 int64 零值: 前端漏填 / 透传丢字段都会得到它, 放行后报的错与真因无关。
		_, err := app.CreateEvaluationSet(context.Background(), &eval_set.CreateEvaluationSetRequest{
			WorkspaceID:       1,
			Name:              gptr.Of("zero-template"),
			TemplateDatasetID: gptr.Of(int64(0)),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "template_dataset_id", "报错要指名道姓")
	})

	t.Run("template_dataset_id 为负 → 拒绝", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, auth, svc := newCreateEvalSetApp(ctrl)
		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Times(0)
		svc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).Times(0)

		_, err := app.CreateEvaluationSet(context.Background(), &eval_set.CreateEvaluationSetRequest{
			WorkspaceID:       1,
			Name:              gptr.Of("negative-template"),
			TemplateDatasetID: gptr.Of(int64(-1)),
		})
		require.Error(t, err)
	})

	t.Run("只传 template_dataset_id 不传 schema → 放行, 且 id 原样下传", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, auth, svc := newCreateEvalSetApp(ctrl)
		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
		svc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *entity.CreateEvaluationSetParam) (int64, error) {
				// 这是本次放宽的正向路径: 模板建集允许不带 schema。
				require.NotNil(t, param.TemplateDatasetID)
				assert.Equal(t, int64(100), *param.TemplateDatasetID,
					"template_dataset_id 必须原样下传 —— 丢了就退化成建一个普通空集")
				assert.Nil(t, param.EvaluationSetSchema, "没传 schema 就该是 nil, 不能凭空造一个空 schema")
				return 7, nil
			})

		resp, err := app.CreateEvaluationSet(context.Background(), &eval_set.CreateEvaluationSetRequest{
			WorkspaceID:       1,
			Name:              gptr.Of("templated"),
			TemplateDatasetID: gptr.Of(int64(100)),
		})
		require.NoError(t, err)
		assert.Equal(t, int64(7), resp.GetEvaluationSetID())
	})

	t.Run("只传 schema 不传模板 → 放行 (存量行为不变)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app, auth, svc := newCreateEvalSetApp(ctrl)
		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
		svc.EXPECT().CreateEvaluationSet(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *entity.CreateEvaluationSetParam) (int64, error) {
				assert.Nil(t, param.TemplateDatasetID, "普通建集不该凭空带上模板 id")
				require.NotNil(t, param.EvaluationSetSchema)
				return 8, nil
			})

		resp, err := app.CreateEvaluationSet(context.Background(), &eval_set.CreateEvaluationSetRequest{
			WorkspaceID:         1,
			Name:                gptr.Of("plain"),
			EvaluationSetSchema: &domain_eval_set.EvaluationSetSchema{},
		})
		require.NoError(t, err)
		assert.Equal(t, int64(8), resp.GetEvaluationSetID())
	})
}

// 改 schema 前必须先过 ValidateEvaluationSetSchemaUpdate, 且**校验失败要中止写入**。
//
// 这个校验是"模板锁定列不许改"的唯一执行点 (开源部署放行, 商业版据当前 schema + 模板判定)。
// 它的两种回归都很贵:
//
//   - 漏调校验 → 模板锁定的列可以被随意改名/改类型, 该评测集从此与模板脱钩,
//     而改动是成功的、数据也还在, 只有后续按模板做的任何操作会开始报奇怪的错;
//   - 调了但不看返回值 → 同上, 且更隐蔽 (代码里"有校验")。
//
// 所以这里既断言"校验被调到且拿到了正确的上下文", 也断言"校验失败时 Update 一次都没被调到"。
func TestEvaluationSetApplicationImpl_UpdateEvaluationSetSchema_ValidatesBeforeWrite(t *testing.T) {
	currentSchema := &entity.EvaluationSetSchema{
		FieldSchemas: []*entity.FieldSchema{{Key: "messages", Locked: true}},
	}
	newFields := []*domain_eval_set.FieldSchema{{Key: gptr.Of("messages"), Name: gptr.Of("改名了")}}

	t.Run("校验通过 → 写入, 且校验拿到当前 schema 与待写字段", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		auth := rpcmocks.NewMockIAuthProvider(ctrl)
		svc := servicemocks.NewMockIEvaluationSetService(ctrl)
		schemaSvc := servicemocks.NewMockEvaluationSetSchemaService(ctrl)
		app := &EvaluationSetApplicationImpl{
			auth:                       auth,
			evaluationSetService:       svc,
			evaluationSetSchemaService: schemaSvc,
		}

		svc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(5), gomock.Any(), gomock.Any()).
			Return(&entity.EvaluationSet{
				ID: 5, SpaceID: 1,
				EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: currentSchema},
			}, nil)
		auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)

		svc.EXPECT().ValidateEvaluationSetSchemaUpdate(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *entity.ValidateEvaluationSetSchemaUpdateParam) error {
				// 校验器必须同时拿到"现状"与"待写" —— 少任一侧它就无法判断 locked 列是否被改。
				assert.Equal(t, int64(1), param.SpaceID)
				assert.Equal(t, int64(5), param.EvaluationSetID)
				require.NotNil(t, param.CurrentSchema, "缺当前 schema 校验器判不出哪些列是 locked")
				require.Len(t, param.CurrentSchema.FieldSchemas, 1)
				assert.True(t, param.CurrentSchema.FieldSchemas[0].Locked)
				require.Len(t, param.UpdatedFieldSchemas, 1)
				assert.Equal(t, "改名了", param.UpdatedFieldSchemas[0].Name)
				return nil
			})

		// 校验通过后写入的必须是**同一份**字段 (不能校验一份、写另一份)。
		schemaSvc.EXPECT().UpdateEvaluationSetSchema(gomock.Any(), int64(1), int64(5), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ int64, fields []*entity.FieldSchema) error {
				require.Len(t, fields, 1)
				assert.Equal(t, "改名了", fields[0].Name)
				return nil
			})

		_, err := app.UpdateEvaluationSetSchema(context.Background(), &eval_set.UpdateEvaluationSetSchemaRequest{
			WorkspaceID: 1, EvaluationSetID: 5, Fields: newFields,
		})
		require.NoError(t, err)
	})

	t.Run("校验失败 → 不写入", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		auth := rpcmocks.NewMockIAuthProvider(ctrl)
		svc := servicemocks.NewMockIEvaluationSetService(ctrl)
		schemaSvc := servicemocks.NewMockEvaluationSetSchemaService(ctrl)
		app := &EvaluationSetApplicationImpl{
			auth:                       auth,
			evaluationSetService:       svc,
			evaluationSetSchemaService: schemaSvc,
		}

		svc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(5), gomock.Any(), gomock.Any()).
			Return(&entity.EvaluationSet{
				ID: 5, SpaceID: 1,
				EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: currentSchema},
			}, nil)
		auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		svc.EXPECT().ValidateEvaluationSetSchemaUpdate(gomock.Any(), gomock.Any()).
			Return(errors.New("locked field cannot be modified"))
		// 关键: 校验失败后写入一次都不能发生, 否则"模板锁列"这条约束等于不存在。
		schemaSvc.EXPECT().UpdateEvaluationSetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		_, err := app.UpdateEvaluationSetSchema(context.Background(), &eval_set.UpdateEvaluationSetSchemaRequest{
			WorkspaceID: 1, EvaluationSetID: 5, Fields: newFields,
		})
		require.Error(t, err)
	})

	t.Run("评测集无版本 → 校验仍要被调用 (CurrentSchema 为 nil)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		auth := rpcmocks.NewMockIAuthProvider(ctrl)
		svc := servicemocks.NewMockIEvaluationSetService(ctrl)
		schemaSvc := servicemocks.NewMockEvaluationSetSchemaService(ctrl)
		app := &EvaluationSetApplicationImpl{
			auth:                       auth,
			evaluationSetService:       svc,
			evaluationSetSchemaService: schemaSvc,
		}

		// 刚建、还没提交过版本的评测集: EvaluationSetVersion 为 nil。
		// 这条防的是"取 CurrentSchema 时漏判空 → panic 500", 以及"版本为空就跳过校验"这种
		// 想当然的短路 (跳过后, 模板集在首次提交版本前的改列窗口里完全不受约束)。
		svc.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(5), gomock.Any(), gomock.Any()).
			Return(&entity.EvaluationSet{ID: 5, SpaceID: 1}, nil)
		auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
		svc.EXPECT().ValidateEvaluationSetSchemaUpdate(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *entity.ValidateEvaluationSetSchemaUpdateParam) error {
				assert.Nil(t, param.CurrentSchema)
				return nil
			})
		schemaSvc.EXPECT().UpdateEvaluationSetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		_, err := app.UpdateEvaluationSetSchema(context.Background(), &eval_set.UpdateEvaluationSetSchemaRequest{
			WorkspaceID: 1, EvaluationSetID: 5, Fields: newFields,
		})
		require.NoError(t, err)
	})
}
