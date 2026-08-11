// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
)

// 长连接 Agent（CustomAgent）「自定义输出字段声明」的创建时前置校验。
// 声明由用户在 fornax 应用管理页填写，经 ListApplicationRPC 注入到
// CustomAgent.CustomFieldSchemas；评测侧在生成 OutputSchema 之前必须自我保护。

const (
	// customFieldNameMaxLen 字段名长度上限。
	//
	// 【铁律：与前端既有实现严格一致，禁止放宽】
	// 前端声明表单输入框写死 maxLength={50}
	// (evaluate-components/src/.../custom-field-schema-config/field-row-render.tsx，界面上显示为 0/50)。
	// 后端若比前端宽，会造成「前端拦得住、OpenAPI 绕得过」：脏数据一旦入库，
	// 用户下次在页面编辑该 Agent 时会因前端校验不过而无法保存（死锁）。
	customFieldNameMaxLen = 50
)

var (
	// customFieldNameRegexp 字段名字符集白名单：字母开头，其后字母/数字/下划线。
	//
	// 【铁律：与前端既有实现严格一致，禁止放宽 —— 尤其不要允许 `_` 开头】
	//  1. 这是对齐前端既有 columnNameRuleValidator
	//     (evaluate-components/src/utils/source-name-rule.ts) 的 /^[a-zA-Z][a-zA-Z0-9_]*$/，
	//     该 validator 被 6+ 处复用，动它会外溢到长连接以外，故我们对齐它、不改它。
	//     后端更宽 => 「前端拦得住、OpenAPI 绕得过」+ 用户再编辑时无法保存（脏数据死锁）。
	//  2. 该规则天然满足 JSONPath 硬约束：字段名会成为 OutputFields 的 key，
	//     下游评估器字段映射用 JSONPath 取值，`.` / `[` / `]` / `$` 会被当路径分隔符，
	//     本正则已排除全部这些字符。
	customFieldNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

	// reservedCustomFieldNames 系统保留字段名，用户不得声明（引用常量，不写字面量）。
	reservedCustomFieldNames = map[string]struct{}{
		consts.EvalTargetOutputFieldKeyActualOutput:       {},
		consts.EvalTargetOutputFieldKeyTrajectory:         {},
		consts.EvalTargetOutputFieldKeyScreenRecordingURI: {},
		consts.EvalTargetOutputFieldKeyScreenRecordingURL: {},
	}

	// allowedCustomFieldScalarSchemaKeys 可声明的标量类型白名单。
	//
	// 仅 String/Integer/Float/Bool 四类标量 + 多模态（由 ContentType == MultiPart 表达）。
	// 显式不开放：
	//   - Object / Array：无对应 SchemaKey，落到「缺少字段类型」或白名单外，均被拦截；
	//   - Message(5) / SingleChoice(6) / MessageList(8)：非本链路语义；
	//   - Trajectory(7)：必须拦。CustomAgent 已自动追加 trajectory 列
	//     (target_source_custom_agent_impl.go)，报告侧 (expt_result_impl.go) 还会独立追加一次，
	//     而 buildOutputSchema 对 name == trajectory 直接 continue 跳过
	//     => 用户手工声明只会被静默丢弃，拦截可避免「填了没效果」的困惑。
	allowedCustomFieldScalarSchemaKeys = map[SchemaKey]struct{}{
		SchemaKey_String:  {},
		SchemaKey_Integer: {},
		SchemaKey_Float:   {},
		SchemaKey_Bool:    {},
	}
)

// ValidateCustomFieldSchemas 校验长连接 Agent 自定义输出字段声明。
//
// 规则（失败即拦截，不 fallback、不静默去重、不以默认类型静默补全）：
//   - R1 声明整体可空：nil / 空切片放行（OutputSchema 退化为仅 actual_output）
//   - R2 字段名非空
//   - R3 字段名不含任意 unicode 空白（含全角空格 U+3000）
//   - R4 字段名匹配 ^[a-zA-Z][a-zA-Z0-9_]*$（不允许 `_` 开头）
//   - R5 字段名长度 <= 50（按 rune 计）
//   - R6 不得使用系统保留字
//   - R7 同批声明内不得重名
//   - R8 类型必填且在白名单内（4 类标量 + 多模态）
//   - R8b 显式拒绝 Object / Array / Trajectory
//
// 错误一律为 CommonInvalidParamCode，且错误信息必含出错字段名，便于用户自查。
func ValidateCustomFieldSchemas(schemas []*CustomFieldSchema) error {
	// R1：整体可空，最先短路。
	if len(schemas) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(schemas))
	for i, s := range schemas {
		// nil 元素 fail-loud：不静默跳过，否则用户声明被吞掉且无从察觉。
		if s == nil {
			return invalidParam(fmt.Sprintf("custom_field_schemas[%d] is nil", i))
		}

		// R2：字段名非空。
		if strings.TrimSpace(s.Name) == "" {
			return invalidParam(fmt.Sprintf("custom_field_schemas[%d]: 自定义输出字段名不能为空", i))
		}
		// R3：不含任意 unicode 空白（空格/Tab/换行/全角空格 U+3000）。
		if strings.ContainsFunc(s.Name, unicode.IsSpace) {
			return invalidParam(fmt.Sprintf("自定义输出字段名 %q 不能包含空白字符", s.Name))
		}
		// R4：字符集与首字符。
		if !customFieldNameRegexp.MatchString(s.Name) {
			return invalidParam(fmt.Sprintf("自定义输出字段名 %q 不合法：仅支持字母/数字/下划线，且必须以字母开头", s.Name))
		}
		// R5：长度上限（按 rune 计，避免多字节字符按 byte 误判）。
		if utf8.RuneCountInString(s.Name) > customFieldNameMaxLen {
			return invalidParam(fmt.Sprintf("自定义输出字段名 %q 超出长度限制（最多 %d 字符）", s.Name, customFieldNameMaxLen))
		}
		// R6：保留字冲突。
		if _, ok := reservedCustomFieldNames[s.Name]; ok {
			return invalidParam(fmt.Sprintf("自定义输出字段名 %q 为系统保留字，请更换", s.Name))
		}
		// R7：同批重名，不去重、不后者覆盖。
		if _, ok := seen[s.Name]; ok {
			return invalidParam(fmt.Sprintf("自定义输出字段名 %q 重复声明", s.Name))
		}
		seen[s.Name] = struct{}{}

		// R8 / R8b：类型必填且在白名单内。
		if err := validateCustomFieldSchemaType(s); err != nil {
			return err
		}
	}

	return nil
}

// validateCustomFieldSchemaType 校验单个声明的类型（R8 / R8b）。
//
// 多模态通过 ContentType == MultiPart 表达，此时 SchemaKey 允许为空；
// 但若同时显式带了 SchemaKey，仍须落在标量白名单内，否则 Trajectory 等
// 不开放的类型可以借 MultiPart 分支绕过白名单。
func validateCustomFieldSchemaType(s *CustomFieldSchema) error {
	switch {
	case s.ContentType == ContentTypeMultipart:
		if s.SchemaKey == nil {
			return nil
		}
		if _, ok := allowedCustomFieldScalarSchemaKeys[*s.SchemaKey]; !ok {
			return unsupportedCustomFieldType(s.Name, *s.SchemaKey)
		}
		return nil
	case s.SchemaKey == nil:
		return invalidParam(fmt.Sprintf("自定义输出字段 %q 缺少字段类型", s.Name))
	default:
		if _, ok := allowedCustomFieldScalarSchemaKeys[*s.SchemaKey]; !ok {
			return unsupportedCustomFieldType(s.Name, *s.SchemaKey)
		}
		return nil
	}
}

func unsupportedCustomFieldType(name string, key SchemaKey) error {
	return invalidParam(fmt.Sprintf("自定义输出字段 %q 的类型 %s 不受支持，当前仅支持 文本/整数/浮点/布尔/多模态", name, key.String()))
}
