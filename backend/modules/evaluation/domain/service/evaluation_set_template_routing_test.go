// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 模板路由的三条边界: nil 参数、Noop 实现、以及"用了模板则请求里的 schema 不作数"。
//
// 开源部署刻意用 Noop 实现暴露空目录并拒绝模板建集 —— 这不是占位代码, 它是**开源/商业行为
// 边界的定义**: 若哪天 Noop 的 Build 从"报错"退化成"返回 nil, nil", 开源部署下带
// template_dataset_id 的建集请求会**建出一个没有任何列的评测集**并返回成功, 用户后续导入数据
// 全部字段不匹配, 而错误发生在离根因很远的地方。

// ValidateEvaluationSetSchemaUpdate 是"改列"路径上的准入校验。
//
// Noop 放行(返回 nil)是刻意的: 开源部署没有模板, 也就没有"模板列不许改"这条约束。
// 这条测试钉住那个刻意的放行 —— 若哪天 Noop 改成报错, 开源部署下**所有**改 schema 的操作
// (与模板毫无关系的普通评测集也算) 会全部被拒, 那是个显性但影响面很大的回归。
func TestNoopEvaluationSetTemplateService_ValidateAlwaysAllows(t *testing.T) {
	t.Parallel()

	svc := NewNoopEvaluationSetTemplateService()
	ctx := context.Background()

	// nil 参数也不该 panic —— 校验器是最外层调用的第一跳。
	assert.NoError(t, svc.ValidateEvaluationSetSchemaUpdate(ctx, nil))
	assert.NoError(t, svc.ValidateEvaluationSetSchemaUpdate(ctx, &entity.ValidateEvaluationSetSchemaUpdateParam{
		SpaceID:         1,
		EvaluationSetID: 2,
		CurrentSchema: &entity.EvaluationSetSchema{
			FieldSchemas: []*entity.FieldSchema{{Key: "messages", Locked: true}},
		},
		// 即使把一个 locked 列改掉, 开源部署也放行 (无模板即无此约束)。
		UpdatedFieldSchemas: []*entity.FieldSchema{{Key: "messages", Name: "改名了"}},
	}), "开源部署没有模板约束, 改列必须放行; 若这里报错, 普通评测集也改不了 schema")
}

// Noop 的 List 必须返回**空目录 + total=0 + nextPageToken=nil**, 而不是 nil 切片配 nil total。
//
// 上层 ListEvaluationSetTemplates 直接把 total / nextPageToken 塞进响应:
// total 为 nil 时前端分页组件拿不到总数(通常渲染成 loading 或 NaN); nextPageToken 非 nil
// 则前端会认为"还有下一页"并无限翻页。空目录的正确表达是"有 total 且为 0, 无 next token"。
func TestNoopEvaluationSetTemplateService_ListReturnsEmptyCatalogNotNilTotal(t *testing.T) {
	t.Parallel()

	templates, total, nextPageToken, err := NewNoopEvaluationSetTemplateService().
		ListEvaluationSetTemplates(context.Background(), &entity.ListEvaluationSetTemplatesParam{SpaceID: 1})
	require.NoError(t, err)
	assert.Empty(t, templates)
	require.NotNil(t, total, "total 必须非 nil —— 前端分页拿不到总数会渲染异常")
	assert.Equal(t, int64(0), *total)
	assert.Nil(t, nextPageToken, "空目录不该给 next token, 否则前端会无限翻页")
}

// Noop 的 Build 必须**报错**而不是返回空 schema。
//
// 这是开源/商业边界上唯一的硬约束: 返回 (nil, nil) 会让 CreateEvaluationSet 拿着
// EvaluationSetSchema=nil 一路走到 CreateDataset, 建出一个零列评测集并返回成功。
// 用户要到往里导数据时才发现所有字段都对不上, 那时已经离根因很远。
func TestNoopEvaluationSetTemplateService_BuildRejectsInsteadOfReturningEmptySchema(t *testing.T) {
	t.Parallel()

	schema, err := NewNoopEvaluationSetTemplateService().BuildEvaluationSetSchemaFromTemplate(
		context.Background(), &entity.BuildEvaluationSetSchemaFromTemplateParam{SpaceID: 1, TemplateDatasetID: 100})
	require.Error(t, err, "不支持模板时必须报错; 返回空 schema 会建出零列评测集并 success")
	assert.Nil(t, schema)
}

// 模板建集时, 请求里带的 schema 只是**建议**, 权威结果由模板服务产出。
//
// 这一跳的语义是"用模板则以模板为准": 请求 schema 作为 RequestedSchema 传给模板服务
// (由它决定是否接受用户追加的列), 而最终写库用的是**模板服务的返回值**。
// 若这里退化成"用请求 schema", 模板锁定的列就会被用户请求覆盖 —— 模板等于失效,
// 但建集成功、列看着也有, 只是与模板脱钩。
func TestEvaluationSetServiceImpl_TemplateSchemaWinsOverRequestedSchema(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	adapter := rpcmocks.NewMockIDatasetRPCAdapter(ctrl)
	templateID := int64(200)

	requested := &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{{Key: "user_added"}}}
	authoritative := &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{
		{Key: "messages", Locked: true},
		{Key: "user_added"},
	}}
	templateSvc := &fakeEvaluationSetTemplateService{builtSchema: authoritative}
	svc := &EvaluationSetServiceImpl{datasetRPCAdapter: adapter, templateService: templateSvc}

	adapter.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *rpc.CreateDatasetParam) (int64, error) {
			assert.Same(t, authoritative, param.EvaluationSetItems,
				"落库 schema 必须来自模板服务, 不能是请求里那份 —— 否则模板锁定的列会被用户覆盖")
			// template_dataset_id 必须一起落库: 它是"这个集由哪个模板建的"的唯一记录,
			// 丢了之后再也无法按模板校验后续的改列操作。
			require.NotNil(t, param.TemplateDatasetID)
			assert.Equal(t, templateID, *param.TemplateDatasetID)
			return 300, nil
		})

	id, err := svc.CreateEvaluationSet(context.Background(), &entity.CreateEvaluationSetParam{
		SpaceID:             1,
		Name:                "templated",
		TemplateDatasetID:   &templateID,
		EvaluationSetSchema: requested,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(300), id)

	// 请求 schema 必须作为 RequestedSchema 传下去 (模板服务据此决定能否追加用户列);
	// 传丢了, "模板 + 用户自加列"这种用法就静默退化成"只有模板列"。
	require.NotNil(t, templateSvc.buildParam)
	assert.Same(t, requested, templateSvc.buildParam.RequestedSchema)
	assert.Equal(t, int64(1), templateSvc.buildParam.SpaceID)
	assert.Equal(t, templateID, templateSvc.buildParam.TemplateDatasetID)
}

// 模板服务报错必须**中断建集**, 不能兜底建一个非模板集。
//
// 兜底的后果: 用户点"用模板建集"、拿到成功和一个 id, 但那个集与模板毫无关系
// (列是用户请求里那份, 且 template_dataset_id 没落库)。显性报错远好于建出一个错的集。
func TestEvaluationSetServiceImpl_TemplateBuildErrorAbortsCreate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	adapter := rpcmocks.NewMockIDatasetRPCAdapter(ctrl)
	// 关键断言: CreateDataset 一次都不能被调到。
	adapter.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).Times(0)

	svc := &EvaluationSetServiceImpl{
		datasetRPCAdapter: adapter,
		// Noop 实现的 Build 必然报错, 正好当作"模板服务不可用"的替身。
		templateService: NewNoopEvaluationSetTemplateService(),
	}

	id, err := svc.CreateEvaluationSet(context.Background(), &entity.CreateEvaluationSetParam{
		SpaceID:           1,
		Name:              "templated",
		TemplateDatasetID: gptr.Of(int64(200)),
	})
	require.Error(t, err, "模板取不到时必须失败, 不能兜底建一个与模板无关的评测集")
	assert.Zero(t, id)
}

// 非模板建集必须**清掉客户端传来的 locked 标记**。
//
// locked 是服务端概念(模板锁列), 客户端不该有权设置。若原样落库, 任何调用方都能给自建评测集
// 的列打上 locked ⇒ 那些列从此在页面上不可编辑, 而它们并不属于任何模板, 也就没有任何
// 合法途径能解锁 —— 数据被永久写死, 且当时建集是成功的。
func TestEvaluationSetServiceImpl_NonTemplateCreateStripsClientLocked(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	adapter := rpcmocks.NewMockIDatasetRPCAdapter(ctrl)
	svc := &EvaluationSetServiceImpl{
		datasetRPCAdapter: adapter,
		templateService:   NewNoopEvaluationSetTemplateService(),
	}

	adapter.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *rpc.CreateDatasetParam) (int64, error) {
			require.Len(t, param.EvaluationSetItems.FieldSchemas, 3)
			for i, f := range param.EvaluationSetItems.FieldSchemas {
				if f == nil {
					continue
				}
				assert.Falsef(t, f.Locked,
					"第 %d 列的 locked 未被清除 —— 客户端将能把自建评测集的列永久写死", i)
			}
			assert.Nil(t, param.TemplateDatasetID, "非模板建集不该落 template_dataset_id")
			return 100, nil
		})

	_, err := svc.CreateEvaluationSet(context.Background(), &entity.CreateEvaluationSetParam{
		SpaceID: 1,
		Name:    "plain",
		EvaluationSetSchema: &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{
			{Key: "input", Locked: true},
			// 夹一个 nil 元素: schema 由客户端构造, nil 是现实输入, 清锁时不能 panic。
			nil,
			{Key: "output", Locked: true},
		}},
	})
	require.NoError(t, err)
}

// 上层 List / Validate 的 nil 参数守卫: 必须报参数错误而不是 panic 或穿透到实现层。
//
// 这两个方法都是 handler 直接调的第一跳, nil 穿透进去会变成 500 + 一条 panic 栈,
// 而正确行为是 400 参数错误。
func TestEvaluationSetServiceImpl_NilParamGuards(t *testing.T) {
	t.Parallel()

	svc := &EvaluationSetServiceImpl{templateService: NewNoopEvaluationSetTemplateService()}
	ctx := context.Background()

	_, _, _, err := svc.ListEvaluationSetTemplates(ctx, nil)
	assert.Error(t, err, "nil param 必须报参数错误, 不能 panic")

	assert.Error(t, svc.ValidateEvaluationSetSchemaUpdate(ctx, nil))
}

// List / Validate 必须把参数**原样**交给模板服务, 且返回值原样带回。
//
// 这一跳是纯转发, 唯一的回归面是"转发时改了参数或吞了返回值":
// SpaceID 传错 ⇒ 拿到别的空间的模板目录 (跨空间数据泄漏, 且不报错);
// 分页参数丢失 ⇒ 永远只返回第一页, 用户看不到后面的模板。
func TestEvaluationSetServiceImpl_TemplateCallsForwardParamsAndResults(t *testing.T) {
	t.Parallel()

	total := int64(7)
	nextPageToken := "tok"
	templateSvc := &fakeEvaluationSetTemplateService{
		templates:     []*entity.EvaluationSetTemplate{{TemplateDatasetID: 1}},
		total:         &total,
		nextPageToken: &nextPageToken,
	}
	svc := &EvaluationSetServiceImpl{templateService: templateSvc}

	listParam := &entity.ListEvaluationSetTemplatesParam{
		SpaceID:   42,
		PageSize:  gptr.Of(int32(20)),
		PageToken: gptr.Of("cur"),
	}
	templates, gotTotal, gotToken, err := svc.ListEvaluationSetTemplates(context.Background(), listParam)
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, &total, gotTotal, "total 必须原样带回, 否则前端分页总数不对")
	assert.Equal(t, &nextPageToken, gotToken, "next token 丢了用户就翻不到后面的模板")

	validateParam := &entity.ValidateEvaluationSetSchemaUpdateParam{
		SpaceID:             42,
		EvaluationSetID:     99,
		CurrentSchema:       &entity.EvaluationSetSchema{},
		UpdatedFieldSchemas: []*entity.FieldSchema{{Key: "k"}},
	}
	require.NoError(t, svc.ValidateEvaluationSetSchemaUpdate(context.Background(), validateParam))
	assert.Same(t, validateParam, templateSvc.validateParam,
		"参数必须原样转发 —— SpaceID 被改写会导致跨空间校验/读取")
}
