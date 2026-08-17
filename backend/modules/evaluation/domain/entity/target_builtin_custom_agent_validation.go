// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
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

	// customFieldSchemasMaxCount 单个 Agent 可声明的自定义输出字段数上限。
	//
	// 与 customFieldNameMaxLen 同一条铁律：对齐前端声明表单的 maxColumn = 20
	// (evaluate-components/src/.../custom-field-schema-config/custom-field-schema-config.tsx)。
	// 后端不设限则 OpenAPI 可写入远超 20 条的声明，用户下次在页面编辑该 Agent 时
	// 会因前端上限而无法保存（同一类脏数据死锁）。
	customFieldSchemasMaxCount = 20
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

	// allowedCustomFieldScalarSchemaKeys 「显式带了 SchemaKey」时的标量白名单。
	//
	// 【SchemaKey 在既有范式里的真实职责：只标记 Trajectory 一种特例，不是标量类型的载体】
	// 实证前端既有实现 custom-field-schema-convert.ts:35-43 与
	// field-convert.ts 的 convertDataTypeToSchema：
	//   - String/Integer/Float/Boolean/Object/Array → content_type=text,       schema_key=**undefined**，
	//     类型全部靠 text_schema 里的 JSON Schema 表达
	//   - MultiPart                                → content_type=multi_part,  schema_key=**undefined**
	//   - Trajectory                               → content_type=text,        schema_key=trajectory
	// 后端既有消费侧 (buildOutputSchema, target_source_custom_rpc_server_impl.go) 完全吻合：
	// 仅 SchemaKey != nil 时查 switch（该 switch 无 default、不报错），随后 TextSchema 非空即覆盖。
	//
	// 故这份白名单只在调用方显式带了 SchemaKey 时兜底生效（主要就是拦 Trajectory），
	// **绝不能拿 SchemaKey == nil 当「类型缺失」的判据** —— 那会把前端正常提交的
	// 全部标量字段误杀（本次 P0 的根因之一）。
	//
	// 显式不开放：
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

	// allowedCustomFieldJSONSchemaTypes 非多模态字段 text_schema 顶层 "type" 的白名单。
	//
	// 对齐前端 TYPE_CONFIG (evaluate-components/src/utils/field-convert.ts) 对
	// 长连接 5 项下拉 (custom-agent-data-type-options.ts) 的产出：
	//   String  -> {"type":"string"}    Integer -> {"type":"integer"}
	//   Float   -> {"type":"number"}    Boolean -> {"type":"boolean"}
	// （注意 Float 的 JSON Schema type 是 number，不是 float。）
	//
	// 显式不含 object / array：这是原「不开放 Object / Array」意图在新范式下的落点 ——
	// 既有范式里 Object/Array 与标量一样都是 schema_key=undefined，
	// 已无法靠 SchemaKey 区分，只能看 text_schema 的顶层 type。
	// 前端下拉已收窄为 5 项，这里是「前端拦得住、OpenAPI 绕得过」的后端防线。
	allowedCustomFieldJSONSchemaTypes = map[string]struct{}{
		"string":  {},
		"integer": {},
		"number":  {},
		"boolean": {},
	}
)

// ValidateCustomFieldSchemas 校验长连接 Agent 自定义输出字段声明。
//
// 规则（失败即拦截，不 fallback、不静默去重、不以默认类型静默补全）：
//   - R1 声明整体可空：nil / 空切片放行（OutputSchema 退化为仅 actual_output）
//   - R1b 声明总数 <= 20（对齐前端 maxColumn，封 OpenAPI 绕过）
//   - R2 字段名非空
//   - R3 字段名不含任意 unicode 空白（含全角空格 U+3000）
//   - R4 字段名匹配 ^[a-zA-Z][a-zA-Z0-9_]*$（不允许 `_` 开头）
//   - R5 字段名长度 <= 50（按 rune 计）
//   - R7 同批声明内不得重名
//   - R8 类型可判定且在开放范围内（4 类标量 + 多模态）。判定方式对齐既有范式：
//     ContentType == MultiPart 即多模态；否则以 text_schema 的 JSON Schema 顶层 type 为准；
//     SchemaKey **不是必填**（既有范式下它只标记 Trajectory，标量恒为空）。
//   - R8b 显式拒绝 Object / Array（text_schema type 为 object/array）与 Trajectory（SchemaKey）
//
// 错误一律为 CommonInvalidParamCode，且错误信息必含出错字段名，便于用户自查。
func ValidateCustomFieldSchemas(schemas []*CustomFieldSchema) error {
	// R1：整体可空，最先短路。
	if len(schemas) == 0 {
		return nil
	}
	// R1b：总数上限，先于逐项校验（超限时不必逐项报错）。
	if len(schemas) > customFieldSchemasMaxCount {
		return invalidParam(fmt.Sprintf("自定义输出字段最多声明 %d 个，当前 %d 个", customFieldSchemasMaxCount, len(schemas)))
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
// 【判定顺序严格对齐既有范式（前端 convertDataTypeToSchema + 后端 buildOutputSchema）】
//  1. ContentType == MultiPart  → 多模态，类型即已确定，text_schema 按范式为空，放行
//  2. SchemaKey != nil          → 特例标记（范式里只有 Trajectory 走这条），查白名单拦掉
//  3. 其余                       → 标量，以 text_schema 的 JSON Schema 顶层 type 为准
//
// 【为什么 SchemaKey 不是必填】既有范式下标量的 schema_key 恒为 undefined，
// 类型信息全在 text_schema。把 SchemaKey 当必填会把前端正常提交的
// 全部标量字段误判成「缺少字段类型」——这正是本次 P0。
//
// ⚠️ 前两条分支都**不能**在放行前跳过 text_schema 校验：消费侧
// (target_source_custom_agent_impl.go 的 buildCustomAgentOutputSchema)
// 对 TextSchema 非空即无条件覆盖 JsonSchema，与本字段走了哪条分支无关。
// 若这里放过，object/array 就能借「带 MultiPart」或「带标量 SchemaKey」
// 把 text_schema 夹带进 OutputSchema —— 即 R8b 的两条绕过路径。
func validateCustomFieldSchemaType(s *CustomFieldSchema) error {
	// 1. 多模态：ContentType 自身即类型载体。
	if s.ContentType == ContentTypeMultipart {
		// 多模态若还显式带了 SchemaKey，仍须落在标量白名单内，
		// 否则 Trajectory 等不开放类型可借多模态分支绕过白名单。
		if s.SchemaKey != nil {
			if _, ok := allowedCustomFieldScalarSchemaKeys[*s.SchemaKey]; !ok {
				return unsupportedCustomFieldType(s.Name, *s.SchemaKey)
			}
		}
		// 范式下多模态的 text_schema 为空；非空则它会覆盖 JsonSchema，必须查白名单。
		return validateCustomFieldTextSchemaIfPresent(s)
	}

	// 2. 显式带 SchemaKey：范式里只有 Trajectory 会走到这，查白名单即拦掉。
	if s.SchemaKey != nil {
		if _, ok := allowedCustomFieldScalarSchemaKeys[*s.SchemaKey]; !ok {
			return unsupportedCustomFieldType(s.Name, *s.SchemaKey)
		}
		// 白名单内的标量 key：前端不会产出（恒 undefined），但 OpenAPI 直连可能带，
		// 与 buildOutputSchema 同范式接受它 —— 但 text_schema 一旦非空仍会覆盖
		// JsonSchema，故不能因「类型已由 SchemaKey 表达」就跳过对它的校验。
		return validateCustomFieldTextSchemaIfPresent(s)
	}

	// 3. 标量：类型以 text_schema 的 JSON Schema 顶层 type 为准。
	return validateCustomFieldTextSchema(s)
}

// validateCustomFieldTextSchemaIfPresent 仅在 text_schema 非空时校验它。
//
// 用于「类型已由 ContentType / SchemaKey 确定」的分支：这些分支下 text_schema
// 按范式应为空，但它非空时消费侧会用它覆盖 JsonSchema，故不能不查。
func validateCustomFieldTextSchemaIfPresent(s *CustomFieldSchema) error {
	if strings.TrimSpace(s.TextSchema) == "" {
		return nil
	}
	return validateCustomFieldTextSchema(s)
}

// validateCustomFieldTextSchema 校验标量字段的 text_schema。
//
// text_schema 是既有范式下标量类型的**唯一**载体（schema_key 恒 undefined），
// 因此它缺失/非法/类型不在白名单，都等价于「类型没说清或不受支持」，必须拦。
func validateCustomFieldTextSchema(s *CustomFieldSchema) error {
	if strings.TrimSpace(s.TextSchema) == "" {
		return invalidParam(fmt.Sprintf("自定义输出字段 %q 缺少字段类型", s.Name))
	}

	// 只解析顶层 "type"，不做完整 JSON Schema 校验：
	// 既有消费侧 buildOutputSchema 也只是把 text_schema 原样透传给 JsonSchema，
	// 这里做的是「类型是否在开放范围内」的准入判断，不是 schema 合法性审计。
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(s.TextSchema), &probe); err != nil {
		return invalidParam(fmt.Sprintf("自定义输出字段 %q 的类型定义不是合法 JSON Schema", s.Name))
	}
	if probe.Type == "" {
		return invalidParam(fmt.Sprintf("自定义输出字段 %q 缺少字段类型", s.Name))
	}
	if _, ok := allowedCustomFieldJSONSchemaTypes[probe.Type]; !ok {
		return invalidParam(fmt.Sprintf("自定义输出字段 %q 的类型 %s 不受支持，当前仅支持 文本/整数/浮点/布尔/多模态", s.Name, probe.Type))
	}
	return nil
}

func unsupportedCustomFieldType(name string, key SchemaKey) error {
	return invalidParam(fmt.Sprintf("自定义输出字段 %q 的类型 %s 不受支持，当前仅支持 文本/整数/浮点/布尔/多模态", name, key.String()))
}
