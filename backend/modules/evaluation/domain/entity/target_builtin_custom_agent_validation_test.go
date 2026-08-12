// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
)

// scalarField 构造一个合法的标量声明（默认 String），仅覆写字段名。
//
// 【wire 格式以前端真实提交为准】前端 convertDataTypeToSchema 对标量产出的是
// content_type=text + schema_key=**undefined** + text_schema=JSON Schema。
// 早期版本这里写的是 SchemaKey: gptr.Of(SchemaKey_String)（把 SchemaKey 当类型载体），
// 那是错的假设，直接固化出了 P0：真实请求全被判「缺少字段类型」。
func scalarField(name string) *CustomFieldSchema {
	return &CustomFieldSchema{
		Name:        name,
		ContentType: ContentTypeText,
		TextSchema:  `{"type":"string"}`,
	}
}

// TestValidateCustomFieldSchemas_Invalid UT-E11：R2-R8b 逐条必拦，且错误信息含出错字段名。
func TestValidateCustomFieldSchemas_Invalid(t *testing.T) {
	name51 := strings.Repeat("a", 51)

	cases := []struct {
		name    string
		schemas []*CustomFieldSchema
		// wantMsgContains 断言错误信息中必须出现的片段（通常是出错字段名）
		wantMsgContains string
	}{
		// --- nil 元素 fail-loud ---
		{
			name:            "nil 元素必拦",
			schemas:         []*CustomFieldSchema{nil},
			wantMsgContains: "custom_field_schemas[0]",
		},
		// --- R2 字段名非空 ---
		{
			name:            "R2 空字段名必拦",
			schemas:         []*CustomFieldSchema{scalarField("")},
			wantMsgContains: "不能为空",
		},
		{
			name:            "R2 纯空白字段名必拦",
			schemas:         []*CustomFieldSchema{scalarField("   ")},
			wantMsgContains: "不能为空",
		},
		// --- R3 不含 unicode 空白 ---
		{
			name:            "R3 含半角空格必拦",
			schemas:         []*CustomFieldSchema{scalarField("my field")},
			wantMsgContains: "my field",
		},
		{
			name:            "R3 含全角空格 U+3000 必拦",
			schemas:         []*CustomFieldSchema{scalarField("my　field")},
			wantMsgContains: "空白字符",
		},
		{
			name:            "R3 含 Tab 必拦",
			schemas:         []*CustomFieldSchema{scalarField("my\tfield")},
			wantMsgContains: "空白字符",
		},
		// --- R4 字符集与首字符 ---
		{
			name:            "R4 含点号必拦（JSONPath 分隔符）",
			schemas:         []*CustomFieldSchema{scalarField("my.field")},
			wantMsgContains: "my.field",
		},
		{
			name:            "R4 含左方括号必拦（JSONPath 分隔符）",
			schemas:         []*CustomFieldSchema{scalarField("my[0]")},
			wantMsgContains: "my[0]",
		},
		{
			name:            "R4 数字开头必拦",
			schemas:         []*CustomFieldSchema{scalarField("1abc")},
			wantMsgContains: "1abc",
		},
		{
			name: "R4 下划线开头必拦（对齐前端 columnNameRuleValidator，禁止放宽）",
			// ⚠️ 这一条是从早期草案反转过来的：早期版本曾拟允许 `_` 开头，已被推翻。
			schemas:         []*CustomFieldSchema{scalarField("_field")},
			wantMsgContains: "_field",
		},
		{
			name:            "R4 中文字段名必拦",
			schemas:         []*CustomFieldSchema{scalarField("输出内容")},
			wantMsgContains: "输出内容",
		},
		{
			name:            "R4 含连字符必拦",
			schemas:         []*CustomFieldSchema{scalarField("my-field")},
			wantMsgContains: "my-field",
		},
		// --- R5 长度上限 50 ---
		{
			name:            "R5 51 字符必拦（上限 50）",
			schemas:         []*CustomFieldSchema{scalarField(name51)},
			wantMsgContains: name51,
		},
		// --- R6 保留字（4 个逐一） ---
		{
			name:            "R6 保留字 actual_output 必拦",
			schemas:         []*CustomFieldSchema{scalarField(consts.EvalTargetOutputFieldKeyActualOutput)},
			wantMsgContains: consts.EvalTargetOutputFieldKeyActualOutput,
		},
		{
			name:            "R6 保留字 trajectory 必拦",
			schemas:         []*CustomFieldSchema{scalarField(consts.EvalTargetOutputFieldKeyTrajectory)},
			wantMsgContains: consts.EvalTargetOutputFieldKeyTrajectory,
		},
		{
			name:            "R6 保留字 screen_recording_uri 必拦",
			schemas:         []*CustomFieldSchema{scalarField(consts.EvalTargetOutputFieldKeyScreenRecordingURI)},
			wantMsgContains: consts.EvalTargetOutputFieldKeyScreenRecordingURI,
		},
		{
			name:            "R6 保留字 screen_recording_url 必拦",
			schemas:         []*CustomFieldSchema{scalarField(consts.EvalTargetOutputFieldKeyScreenRecordingURL)},
			wantMsgContains: consts.EvalTargetOutputFieldKeyScreenRecordingURL,
		},
		// --- R7 同批重名 ---
		{
			name:            "R7 同批重名必拦（不去重、不后者覆盖）",
			schemas:         []*CustomFieldSchema{scalarField("dup"), scalarField("dup")},
			wantMsgContains: "重复声明",
		},
		// --- R8 类型可判定 ---
		{
			name: "R8 text_schema 缺失且非 MultiPart 必拦（标量类型无载体）",
			schemas: []*CustomFieldSchema{
				{Name: "no_type", ContentType: ContentTypeText},
			},
			wantMsgContains: "no_type",
		},
		{
			name: "R8 ContentType 与 SchemaKey 与 text_schema 全空必拦",
			schemas: []*CustomFieldSchema{
				{Name: "obj_field"},
			},
			wantMsgContains: "obj_field",
		},
		{
			name: "R8 text_schema 非法 JSON 必拦",
			schemas: []*CustomFieldSchema{
				{Name: "bad_json", ContentType: ContentTypeText, TextSchema: `{"type":`},
			},
			wantMsgContains: "bad_json",
		},
		{
			name: "R8 text_schema 无顶层 type 必拦",
			schemas: []*CustomFieldSchema{
				{Name: "no_top_type", ContentType: ContentTypeText, TextSchema: `{"description":"x"}`},
			},
			wantMsgContains: "no_top_type",
		},
		// --- R8b Object / Array 不开放（新范式下靠 text_schema 顶层 type 判定） ---
		{
			name: "R8b text_schema type=object 必拦（Object 不开放）",
			schemas: []*CustomFieldSchema{
				{Name: "obj_ts", ContentType: ContentTypeText, TextSchema: `{"type":"object","properties":{"a":{"type":"string"}}}`},
			},
			wantMsgContains: "obj_ts",
		},
		{
			name: "R8b text_schema type=array 必拦（Array 不开放）",
			schemas: []*CustomFieldSchema{
				{Name: "arr_ts", ContentType: ContentTypeText, TextSchema: `{"type":"array","items":{"type":"string"}}`},
			},
			wantMsgContains: "arr_ts",
		},
		// --- R8b 白名单外 SchemaKey（显式带 SchemaKey 时的兜底） ---
		{
			name: "R8b SchemaKey=Message(5) 必拦",
			schemas: []*CustomFieldSchema{
				{Name: "msg_field", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Message)},
			},
			wantMsgContains: "msg_field",
		},
		{
			name: "R8b SchemaKey=SingleChoice(6) 必拦",
			schemas: []*CustomFieldSchema{
				{Name: "choice_field", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_SingleChoice)},
			},
			wantMsgContains: "choice_field",
		},
		{
			// 前端 Trajectory 的真实 wire 形态：content_type=text + schema_key=trajectory + 无 text_schema。
			name: "R8b Trajectory(7) 必拦（否则声明被 buildOutputSchema 静默丢弃）",
			schemas: []*CustomFieldSchema{
				{Name: "traj_field", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Trajectory)},
			},
			wantMsgContains: "traj_field",
		},
		{
			name: "R8b Trajectory(7) 不能借 MultiPart 分支绕过白名单",
			schemas: []*CustomFieldSchema{
				{Name: "traj_mp", ContentType: ContentTypeMultipart, SchemaKey: gptr.Of(SchemaKey_Trajectory)},
			},
			wantMsgContains: "traj_mp",
		},
		{
			name: "R8b SchemaKey=MessageList(8) 必拦",
			schemas: []*CustomFieldSchema{
				{Name: "msg_list", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_MessageList)},
			},
			wantMsgContains: "msg_list",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := ValidateCustomFieldSchemas(c.schemas)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), c.wantMsgContains, "错误信息必须包含出错字段名，便于用户自查")
		})
	}
}

// TestValidateCustomFieldSchemas_Valid UT-E12：R1 空声明放行 + 合法用例放行（含 MultiPart、恰好 50 字符）。
func TestValidateCustomFieldSchemas_Valid(t *testing.T) {
	name50 := strings.Repeat("a", 50)

	cases := []struct {
		name    string
		schemas []*CustomFieldSchema
	}{
		// --- R1 整体可空 ---
		{name: "R1 nil 放行", schemas: nil},
		{name: "R1 空切片放行", schemas: []*CustomFieldSchema{}},
		// --- 合法字段名 ---
		{name: "合法字段名 abc", schemas: []*CustomFieldSchema{scalarField("abc")}},
		{name: "合法字段名 a1", schemas: []*CustomFieldSchema{scalarField("a1")}},
		{name: "合法字段名 my_field", schemas: []*CustomFieldSchema{scalarField("my_field")}},
		{name: "合法字段名 大写开头 Abc", schemas: []*CustomFieldSchema{scalarField("Abc")}},
		{name: "恰好 50 字符放行（边界）", schemas: []*CustomFieldSchema{scalarField(name50)}},
		{
			name: "同批不同名放行",
			schemas: []*CustomFieldSchema{
				scalarField("field_a"),
				scalarField("field_b"),
			},
		},
		// --- 4 类标量全部放行（前端真实 wire：schema_key 恒 nil，类型在 text_schema） ---
		{
			name: "4 类标量 String/Integer/Float/Bool 全部放行（schema_key=nil）",
			schemas: []*CustomFieldSchema{
				{Name: "f_string", ContentType: ContentTypeText, TextSchema: `{"type":"string"}`},
				{Name: "f_integer", ContentType: ContentTypeText, TextSchema: `{"type":"integer"}`},
				// ⚠️ Float 的 JSON Schema type 是 number，不是 float（对齐前端 TYPE_CONFIG）。
				{Name: "f_float", ContentType: ContentTypeText, TextSchema: `{"type":"number"}`},
				{Name: "f_bool", ContentType: ContentTypeText, TextSchema: `{"type":"boolean"}`},
			},
		},
		// --- MultiPart 放行（前端真实 wire：schema_key=nil 且无 text_schema） ---
		{
			name: "MultiPart 且 SchemaKey 为 nil、无 text_schema 放行",
			schemas: []*CustomFieldSchema{
				{Name: "f_multipart", ContentType: ContentTypeMultipart},
			},
		},
		{
			name: "MultiPart 且 SchemaKey 在标量白名单内放行",
			schemas: []*CustomFieldSchema{
				{Name: "f_multipart_str", ContentType: ContentTypeMultipart, SchemaKey: gptr.Of(SchemaKey_String)},
			},
		},
		{
			name: "标量 + MultiPart 混合声明放行",
			schemas: []*CustomFieldSchema{
				{Name: "f_scalar", ContentType: ContentTypeText, TextSchema: `{"type":"integer"}`},
				{Name: "f_mp", ContentType: ContentTypeMultipart},
			},
		},
		// --- OpenAPI 直连兼容：显式带白名单内 SchemaKey 且无 text_schema 也放行 ---
		{
			name: "显式 SchemaKey=String 且无 text_schema 放行（OpenAPI 直连，与 buildOutputSchema 同范式）",
			schemas: []*CustomFieldSchema{
				{Name: "f_key_only", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_String)},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.NoError(t, ValidateCustomFieldSchemas(c.schemas))
		})
	}
}

// TestValidateCustomFieldSchemas_FrontendWireFormat 防回归：锁死前端真实提交的 wire 格式必须通过校验。
//
// 【为什么单列一个测试】本次 P0 的直接成因就是「测试固化了错误的 wire 假设」：
// 老测试用 ContentType 驼峰 "Text"/"MultiPart" + SchemaKey 非 nil 构造入参，
// 与前端真实提交（content_type 小写下划线、schema_key 全为 undefined）完全不符，
// 于是校验逻辑把 SchemaKey 当必填也没被测出来。这里显式钉住真实格式。
//
// 事实来源：
//   - 前端 custom-field-schema-convert.ts:35-43（标量/MultiPart/Trajectory 三分支）
//   - 前端 field-convert.ts convertDataTypeToSchema + TYPE_CONFIG（text_schema 取值）
//   - fornax IDL 常量 ContentType_Text="text" / ContentType_MultiPart="multi_part"
//     （经 commercial 侧 convertToContentType 映射成 entity 的 "Text"/"MultiPart" 才进到这里）
func TestValidateCustomFieldSchemas_FrontendWireFormat(t *testing.T) {
	t.Run("content_type=text + schema_key=nil + 合法 text_schema 必须通过（P0 直接回归项）", func(t *testing.T) {
		// 前端 5 项下拉中 4 类标量的完整真实提交形态。
		schemas := []*CustomFieldSchema{
			{Name: "score", ContentType: ContentTypeText, TextSchema: `{"type":"integer"}`},
			{Name: "ratio", ContentType: ContentTypeText, TextSchema: `{"type":"number"}`},
			{Name: "passed", ContentType: ContentTypeText, TextSchema: `{"type":"boolean"}`},
			{Name: "summary", ContentType: ContentTypeText, TextSchema: `{"type":"string"}`},
		}
		assert.NoError(t, ValidateCustomFieldSchemas(schemas),
			"前端标量字段的 schema_key 恒为 undefined，把它当必填即复现 P0「缺少字段类型」")
	})

	t.Run("content_type=multi_part + schema_key=nil 必须被识别为多模态", func(t *testing.T) {
		s := &CustomFieldSchema{Name: "screenshot", ContentType: ContentTypeMultipart}
		// 判定为多模态的可观测表现：无 text_schema 也放行
		// （若被误判成标量，会因 text_schema 为空而报「缺少字段类型」）。
		assert.NoError(t, ValidateCustomFieldSchemas([]*CustomFieldSchema{s}),
			"多模态必须由 ContentType 判定；ContentType 若仍是未映射的 \"multi_part\" 裸值则此处必失败")
		assert.Equal(t, ContentTypeMultipart, s.ContentType,
			"entity 侧多模态常量是驼峰 \"MultiPart\"，fornax 侧是 \"multi_part\"，两者必须经 convertToContentType 映射")
	})

	t.Run("未映射的 fornax 裸字面量会被判为标量并因缺 text_schema 拦下（反向证明映射必要性）", func(t *testing.T) {
		// 若 commercial 侧漏了 convertToContentType，ContentType 会是 "multi_part" 裸值，
		// != ContentTypeMultipart（"MultiPart"），从而落进标量分支。
		err := ValidateCustomFieldSchemas([]*CustomFieldSchema{
			{Name: "screenshot", ContentType: ContentType("multi_part")},
		})
		assert.Error(t, err, "裸 fornax 字面量不应被当成多模态，这正是 P0 需要 convertToContentType 的原因")
	})
}
