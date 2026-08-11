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
func scalarField(name string) *CustomFieldSchema {
	return &CustomFieldSchema{
		Name:        name,
		ContentType: ContentTypeText,
		SchemaKey:   gptr.Of(SchemaKey_String),
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
		// --- R8 类型必填 ---
		{
			name: "R8 SchemaKey 为 nil 且非 MultiPart 必拦",
			schemas: []*CustomFieldSchema{
				{Name: "no_type", ContentType: ContentTypeText},
			},
			wantMsgContains: "no_type",
		},
		{
			name: "R8 ContentType 为空且 SchemaKey 为 nil 必拦（Object/Array 无对应 SchemaKey）",
			schemas: []*CustomFieldSchema{
				{Name: "obj_field"},
			},
			wantMsgContains: "obj_field",
		},
		// --- R8b 白名单外类型 ---
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
			name: "R8b SchemaKey=Trajectory(7) 必拦（否则声明被 buildOutputSchema 静默丢弃）",
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
		// --- 4 类标量全部放行 ---
		{
			name: "4 类标量 String/Integer/Float/Bool 全部放行",
			schemas: []*CustomFieldSchema{
				{Name: "f_string", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_String)},
				{Name: "f_integer", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Integer)},
				{Name: "f_float", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Float)},
				{Name: "f_bool", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Bool)},
			},
		},
		// --- MultiPart 放行（SchemaKey 可空） ---
		{
			name: "MultiPart 且 SchemaKey 为 nil 放行",
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
				{Name: "f_scalar", ContentType: ContentTypeText, SchemaKey: gptr.Of(SchemaKey_Integer)},
				{Name: "f_mp", ContentType: ContentTypeMultipart},
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
