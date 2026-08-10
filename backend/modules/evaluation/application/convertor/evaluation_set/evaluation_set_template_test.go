// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	openapi_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// FieldSchema.Locked 决定前端**能不能改这一列** —— 模板评测集的列由模板锁定, 用户不得改名/改类型。
// 它是纯布尔且 DTO 侧是 *bool, 所以两种回归都不报错、只是权限判断反了:
//
//   - locked=true 丢成 nil/false → 前端放行编辑模板锁定列, 该列与模板脱钩, 后续按模板校验必失败;
//   - locked=false 丢成 nil     → 前端可能按"字段缺失"当作未知态处理, 表现为该列莫名不可编辑。
//
// 两个方向 (DTO2DO 读、DO2DTO 写) 都要钉, 因为两侧此前的默认值处理是不对称的
// (内部 DTO 只在 true 时下发指针, OpenAPI 侧 true/false 都下发)。
func TestFieldSchema_LockedRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("DTO2DO: locked 三态", func(t *testing.T) {
		t.Parallel()
		// 显式 true → 锁定; 显式 false / 字段缺失 → 未锁定 (缺失按未锁定处理是既有缺省语义)。
		assert.True(t, FieldSchemaDTO2DO(&eval_set.FieldSchema{Locked: gptr.Of(true)}).Locked)
		assert.False(t, FieldSchemaDTO2DO(&eval_set.FieldSchema{Locked: gptr.Of(false)}).Locked)
		assert.False(t, FieldSchemaDTO2DO(&eval_set.FieldSchema{}).Locked,
			"locked 缺失必须按未锁定处理, 不能凭空锁死用户自建的列")
	})

	t.Run("DO2DTO: 内部 DTO 只在 true 时下发指针", func(t *testing.T) {
		t.Parallel()
		// 内部 DTO 的 locked 是 optional: 未锁定的列不下发这个键 (json omitempty),
		// 锁定的列必须下发 true —— 丢了它前端就会放行编辑模板列。
		locked := FieldSchemaDO2DTO(&entity.FieldSchema{Key: "messages", Locked: true})
		require.NotNil(t, locked.Locked)
		assert.True(t, *locked.Locked)

		unlocked := FieldSchemaDO2DTO(&entity.FieldSchema{Key: "input"})
		assert.Nil(t, unlocked.Locked, "未锁定列不下发 locked 键 (与内部 DTO 的 optional 语义一致)")
	})

	t.Run("OpenAPI DO2DTO: true/false 都显式下发", func(t *testing.T) {
		t.Parallel()
		// OpenAPI 是对外读模型, 刻意把 false 也显式下发 —— 对外契约上"字段不存在"与
		// "字段为 false"对调用方是两回事, 缺省不下发会让调用方分不清"这个部署不支持 locked"
		// 和"这一列没锁"。这也是本分支的一次 fix (曾只在 true 时下发)。
		require.NotNil(t, OpenAPIFieldSchemaDO2DTO(&entity.FieldSchema{Key: "input"}).Locked)
		assert.False(t, *OpenAPIFieldSchemaDO2DTO(&entity.FieldSchema{Key: "input"}).Locked)
		assert.True(t, *OpenAPIFieldSchemaDO2DTO(&entity.FieldSchema{Key: "m", Locked: true}).Locked)
	})

	t.Run("OpenAPI DTO2DO", func(t *testing.T) {
		t.Parallel()
		assert.True(t, OpenAPIFieldSchemaDTO2DO(&openapi_eval_set.FieldSchema{Locked: gptr.Of(true)}).Locked)
		assert.False(t, OpenAPIFieldSchemaDTO2DO(&openapi_eval_set.FieldSchema{}).Locked)
	})

	t.Run("整份 schema 逐列保持各自的 locked", func(t *testing.T) {
		t.Parallel()
		// 模板评测集的典型形态: 模板列锁定 + 用户自加列不锁定。若转换里把 locked 写成
		// 整份 schema 的统一值(常见的循环变量复用 bug), 混合 schema 就会整份锁死或整份放开。
		dos := []*entity.FieldSchema{
			{Key: "messages", Locked: true},
			{Key: "user_note"},
			{Key: "reference", Locked: true},
		}
		dtos := FieldSchemaDO2DTOs(dos)
		require.Len(t, dtos, 3)
		assert.True(t, gptr.Indirect(dtos[0].Locked))
		assert.Nil(t, dtos[1].Locked)
		assert.True(t, gptr.Indirect(dtos[2].Locked))

		// 回程也逐列保持 (DTO2DO 是编辑保存那条路)。
		back := FieldSchemaDTO2DOs(dtos)
		require.Len(t, back, 3)
		assert.True(t, back[0].Locked)
		assert.False(t, back[1].Locked)
		assert.True(t, back[2].Locked)
	})
}

// FieldSchemaDTO2DO / DO2DTO 的 nil 与空切片边界。
// nil vs 空切片在这里不是洁癖: 上游用 `FieldSchemas == nil` 区分"没传 schema"(不改)
// 与"传了空 schema"(清空所有列), 转换层把 nil 变成 `[]` 会让"不改"变成"清空"。
func TestFieldSchemaConvertors_NilAndEmptyBoundaries(t *testing.T) {
	t.Parallel()

	assert.Nil(t, FieldSchemaDTO2DO(nil))
	assert.Nil(t, FieldSchemaDO2DTO(nil))
	assert.Nil(t, FieldSchemaDTO2DOs(nil), "nil 必须原样返回 nil, 不能变成空切片")
	assert.Nil(t, FieldSchemaDO2DTOs(nil))
	assert.Empty(t, FieldSchemaDTO2DOs([]*eval_set.FieldSchema{}))
	assert.NotNil(t, FieldSchemaDTO2DOs([]*eval_set.FieldSchema{}), "空切片保持空切片")
	assert.Empty(t, FieldSchemaDO2DTOs([]*entity.FieldSchema{}))
	assert.NotNil(t, FieldSchemaDO2DTOs([]*entity.FieldSchema{}))

	// 切片里夹 nil 元素: 不能 panic (上游 DTO 由调用方构造, 夹 nil 是现实输入)。
	assert.Len(t, FieldSchemaDTO2DOs([]*eval_set.FieldSchema{nil, {Key: gptr.Of("k")}}), 2)
	assert.Len(t, FieldSchemaDO2DTOs([]*entity.FieldSchema{nil, {Key: "k"}}), 2)
}

// FieldSchemaDTO2DO 的**全字段**搬运。
//
// 这一跳是"用户在页面上改列定义 → 落库"的唯一路径, 且它是逐字段手写赋值:
// 漏搬一个字段既不报错也不影响其它字段, 现象只是"我改的这一项没保存"。整体比对(而非逐字段
// assert)是为了让**新增字段忘记接线**也 FAIL —— 新字段静默为零值是这类转换器的典型回归。
func TestFieldSchemaDTO2DO_CarriesEveryField(t *testing.T) {
	t.Parallel()

	dto := &eval_set.FieldSchema{
		Key:         gptr.Of("messages"),
		Name:        gptr.Of("对话"),
		Description: gptr.Of("多轮对话列"),
		TextSchema:  gptr.Of(`{"type":"string"}`),
		Hidden:      gptr.Of(true),
		IsRequired:  gptr.Of(true),
		Locked:      gptr.Of(true),
	}
	do := FieldSchemaDTO2DO(dto)
	require.NotNil(t, do)

	assert.Equal(t, "messages", do.Key)
	assert.Equal(t, "对话", do.Name)
	assert.Equal(t, "多轮对话列", do.Description)
	assert.Equal(t, `{"type":"string"}`, do.TextSchema)
	assert.True(t, do.Hidden)
	assert.True(t, do.IsRequired)
	assert.True(t, do.Locked)
	// MultiModelSpec 未传时必须是 nil, 不能是空 struct —— 下游用 nil 判"这一列不支持多模态",
	// 空 struct 会被当成"支持多模态但限额全为 0", 于是任何附件都超限。
	assert.Nil(t, do.MultiModelSpec)
}

// EvaluationSetTemplate 是模板列表接口的返回体; 五个字段各自的静默失效面:
//
//   - template_dataset_id 丢 → 前端拿不到 id, "用此模板建集"直接建不出来 (显性);
//   - is_editable 丢/反   → 不可编辑的模板被放行编辑 (用户改完保存时才被后端拒), 或
//     可编辑模板被锁死 —— **这是本分支专门补的字段**, 反了就是权限反了;
//   - evaluation_set_schema 丢 → 模板预览空列, 用户看不到模板长什么样。
//
// is_editable 用 gptr.Of 无条件下发 (含 false): 对外/对前端契约上"字段缺失"与"false"必须可区分。
func TestEvaluationSetTemplateDO2DTO(t *testing.T) {
	t.Parallel()

	do := &entity.EvaluationSetTemplate{
		TemplateDatasetID:   100,
		TemplateDatasetName: "多轮对话模板",
		Description:         "带 messages 列",
		IsEditable:          true,
		EvaluationSetSchema: &entity.EvaluationSetSchema{
			FieldSchemas: []*entity.FieldSchema{{Key: "messages", Name: "对话", Locked: true}},
		},
	}

	t.Run("内部 DTO", func(t *testing.T) {
		t.Parallel()
		dto := EvaluationSetTemplateDO2DTO(do)
		require.NotNil(t, dto)
		assert.Equal(t, int64(100), dto.GetTemplateDatasetID())
		assert.Equal(t, "多轮对话模板", dto.GetTemplateDatasetName())
		assert.Equal(t, "带 messages 列", dto.GetDescription())
		require.NotNil(t, dto.IsEditable)
		assert.True(t, dto.GetIsEditable())
		// 模板 schema 必须带下来, 且列上的 locked 要透到最外层 (前端据此禁用输入框)。
		require.NotNil(t, dto.EvaluationSetSchema)
		require.Len(t, dto.EvaluationSetSchema.FieldSchemas, 1)
		assert.True(t, dto.EvaluationSetSchema.FieldSchemas[0].GetLocked())
	})

	t.Run("OpenAPI DTO", func(t *testing.T) {
		t.Parallel()
		dto := OpenAPIEvaluationSetTemplateDO2DTO(do)
		require.NotNil(t, dto)
		assert.Equal(t, int64(100), dto.GetTemplateDatasetID())
		assert.Equal(t, "多轮对话模板", dto.GetTemplateDatasetName())
		assert.Equal(t, "带 messages 列", dto.GetDescription())
		require.NotNil(t, dto.IsEditable)
		assert.True(t, dto.GetIsEditable())
		require.NotNil(t, dto.EvaluationSetSchema)
		require.Len(t, dto.EvaluationSetSchema.FieldSchemas, 1)
		assert.True(t, dto.EvaluationSetSchema.FieldSchemas[0].GetLocked())
	})

	t.Run("is_editable=false 必须显式下发", func(t *testing.T) {
		t.Parallel()
		// false 被 omit 掉时前端只能猜, 通常猜成"可编辑" —— 恰好与真实语义相反。
		locked := &entity.EvaluationSetTemplate{TemplateDatasetID: 1, IsEditable: false}
		internal := EvaluationSetTemplateDO2DTO(locked)
		require.NotNil(t, internal.IsEditable)
		assert.False(t, *internal.IsEditable)
		oapi := OpenAPIEvaluationSetTemplateDO2DTO(locked)
		require.NotNil(t, oapi.IsEditable)
		assert.False(t, *oapi.IsEditable)
	})

	t.Run("无 schema 的模板不 panic", func(t *testing.T) {
		t.Parallel()
		// 商业版之外的部署 / 模板刚建还没配列时 schema 为 nil, 列表接口不该整个挂掉。
		bare := &entity.EvaluationSetTemplate{TemplateDatasetID: 7}
		assert.Nil(t, EvaluationSetTemplateDO2DTO(bare).EvaluationSetSchema)
		assert.Nil(t, OpenAPIEvaluationSetTemplateDO2DTO(bare).EvaluationSetSchema)
	})
}

// 模板列表的批量转换边界。
//
// 关键一条: **nil DO 元素必须被跳过而不是产出 nil DTO**。开源部署的模板服务返回空目录,
// 商业版的模板来自 RPC —— 列表里夹一个 nil(单条模板查询失败) 时, 若产出 nil DTO,
// thrift 序列化整个 response 会失败, 表现为"模板列表接口 500", 而实际只是一条模板有问题。
func TestEvaluationSetTemplateDO2DTOs_SkipsNilElements(t *testing.T) {
	t.Parallel()

	assert.Nil(t, EvaluationSetTemplateDO2DTOs(nil))
	assert.Nil(t, OpenAPIEvaluationSetTemplateDO2DTOs(nil))
	assert.Nil(t, EvaluationSetTemplateDO2DTO(nil))
	assert.Nil(t, OpenAPIEvaluationSetTemplateDO2DTO(nil))

	// 开源部署返回空目录: 空切片保持空切片 (不能变 nil, 否则 has_more 之类的判断会漂)。
	assert.Empty(t, EvaluationSetTemplateDO2DTOs([]*entity.EvaluationSetTemplate{}))
	assert.NotNil(t, EvaluationSetTemplateDO2DTOs([]*entity.EvaluationSetTemplate{}))

	dos := []*entity.EvaluationSetTemplate{nil, {TemplateDatasetID: 1}, nil, {TemplateDatasetID: 2}}

	internal := EvaluationSetTemplateDO2DTOs(dos)
	require.Len(t, internal, 2, "nil 元素必须被跳过, 否则序列化整个响应会失败")
	assert.Equal(t, int64(1), internal[0].GetTemplateDatasetID())
	assert.Equal(t, int64(2), internal[1].GetTemplateDatasetID())

	oapi := OpenAPIEvaluationSetTemplateDO2DTOs(dos)
	require.Len(t, oapi, 2)
	assert.Equal(t, int64(1), oapi[0].GetTemplateDatasetID())
	assert.Equal(t, int64(2), oapi[1].GetTemplateDatasetID())
}

// EvaluationSet.template_dataset_id 是"这个评测集由哪个模板创建"的回显。
//
// 它必须**保持指针语义原样透传**: nil = 用户自建的普通评测集, 非 nil = 模板集。
// 若转换里做了 gptr.Of(gptr.Indirect(...)) 之类的解包再包, nil 就变成了指向 0 的指针,
// 前端 `if (template_dataset_id)` 判不出来、把普通评测集当模板集渲染(列被误判为锁定)。
func TestEvaluationSetDO2DTO_TemplateDatasetIDPointerSemantics(t *testing.T) {
	t.Parallel()

	t.Run("普通评测集: nil 保持 nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, EvaluationSetDO2DTO(&entity.EvaluationSet{ID: 1}).TemplateDatasetID,
			"自建评测集不能回显出 template_dataset_id=0, 否则前端会当成模板集")
		assert.Nil(t, OpenAPIEvaluationSetDO2DTO(&entity.EvaluationSet{ID: 1}).TemplateDatasetID)
	})

	t.Run("模板集: 值原样带出", func(t *testing.T) {
		t.Parallel()
		do := &entity.EvaluationSet{ID: 1, TemplateDatasetID: gptr.Of(int64(100))}
		assert.Equal(t, int64(100), EvaluationSetDO2DTO(do).GetTemplateDatasetID())
		assert.Equal(t, int64(100), OpenAPIEvaluationSetDO2DTO(do).GetTemplateDatasetID())
	})
}
