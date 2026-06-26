// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"regexp"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// item-centric 多评测集 (MultiSetConfig) 创建入参的应用层校验。
// 对应《[Vibe]技术方案细化》「校验规则（应用层）」章节，在 eval_set_configs 落库前一次性拦截非法输入。

const (
	// filter 白名单
	maxFilterFields = 10 // 单个 filter 的 filter_fields 数量上限
	maxAliasLen     = 64  // alias 长度上限 (对齐 expt_evaluator_ref.alias VARCHAR(64))
)

var (
	// alias 字符集白名单 [a-zA-Z0-9_-]
	aliasPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// filter 字段白名单
	allowedFilterFieldNames = map[string]struct{}{"item_id": {}}                                              // tag key 走 field_type=tag 放行
	allowedFilterFieldTypes = map[string]struct{}{"long": {}, "tag": {}}                                       // long(item_id) / tag
	allowedFilterQueryTypes = map[string]struct{}{"eq": {}, "not_eq": {}, "in": {}, "not_in": {}}              // 单层比较
)

// ValidateEvalSetConfigs 校验新路径 (MultiSetConfig) 的多评测集配置。
//   - (eval_set_id, eval_set_version_id) 在 request 内不重复
//   - 每个 set 内 (evaluator_version_id, alias) 唯一
//   - target_confs 本期 len<=1, alias 必须空
//   - item_filter / evaluator filter 走白名单 (字段/操作符/单层不嵌套/数量上限)
//   - alias 字符集与长度
//
// 返回 CommonInvalidParamCode 参数错误；configs 为空时视为老路径，直接放行。
func ValidateEvalSetConfigs(configs []*EvalSetConfig) error {
	if len(configs) == 0 {
		return nil
	}

	seenSet := make(map[string]struct{}, len(configs))
	for si, sc := range configs {
		if sc == nil {
			return invalidParam(fmt.Sprintf("eval_set_configs[%d] is nil", si))
		}
		if sc.EvalSetID == 0 || sc.EvalSetVersionID == 0 {
			return invalidParam(fmt.Sprintf("eval_set_configs[%d]: eval_set_id and eval_set_version_id are required", si))
		}
		// set 去重
		setKey := fmt.Sprintf("%d#%d", sc.EvalSetID, sc.EvalSetVersionID)
		if _, ok := seenSet[setKey]; ok {
			return invalidParam(fmt.Sprintf("duplicate (eval_set_id=%d, eval_set_version_id=%d) in eval_set_configs", sc.EvalSetID, sc.EvalSetVersionID))
		}
		seenSet[setKey] = struct{}{}

		// item_filter 白名单
		if err := validateFilter(sc.ItemFilter, fmt.Sprintf("eval_set_configs[%d].item_filter", si)); err != nil {
			return err
		}

		// target_confs: 本期 len<=1, alias 必须空
		if len(sc.TargetConfs) > 1 {
			return invalidParam(fmt.Sprintf("eval_set_configs[%d]: target_confs len must be <= 1, got %d", si, len(sc.TargetConfs)))
		}
		for ti, tc := range sc.TargetConfs {
			if tc == nil {
				continue
			}
			if tc.Alias != "" {
				return invalidParam(fmt.Sprintf("eval_set_configs[%d].target_confs[%d]: alias must be empty in current version", si, ti))
			}
		}

		// evaluator_confs: (version_id, alias) set 内唯一 + alias 合法 + filter 白名单
		seenEvaluator := make(map[string]struct{}, len(sc.EvaluatorConfs))
		for ei, ec := range sc.EvaluatorConfs {
			if ec == nil {
				return invalidParam(fmt.Sprintf("eval_set_configs[%d].evaluator_confs[%d] is nil", si, ei))
			}
			if ec.EvaluatorVersionID == 0 {
				return invalidParam(fmt.Sprintf("eval_set_configs[%d].evaluator_confs[%d]: evaluator_version_id is required", si, ei))
			}
			if err := validateAlias(ec.Alias, fmt.Sprintf("eval_set_configs[%d].evaluator_confs[%d].alias", si, ei)); err != nil {
				return err
			}
			evKey := fmt.Sprintf("%d#%s", ec.EvaluatorVersionID, ec.Alias)
			if _, ok := seenEvaluator[evKey]; ok {
				return invalidParam(fmt.Sprintf("duplicate (evaluator_version_id=%d, alias=%q) in eval_set_configs[%d]", ec.EvaluatorVersionID, ec.Alias, si))
			}
			seenEvaluator[evKey] = struct{}{}

			if err := validateFilter(ec.Filter, fmt.Sprintf("eval_set_configs[%d].evaluator_confs[%d].filter", si, ei)); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateFilter 对 item_filter / evaluator filter 套用白名单：字段名/类型/操作符白名单、单层不嵌套、数量上限、点选基本校验。
func validateFilter(f *ExptItemFilter, path string) error {
	if f == nil {
		return nil
	}
	if len(f.FilterFields) == 0 {
		return invalidParam(fmt.Sprintf("%s: filter_fields must not be empty when filter is set", path))
	}
	if len(f.FilterFields) > maxFilterFields {
		return invalidParam(fmt.Sprintf("%s: filter_fields exceeds max %d", path, maxFilterFields))
	}
	for fi, ff := range f.FilterFields {
		if ff == nil {
			return invalidParam(fmt.Sprintf("%s.filter_fields[%d] is nil", path, fi))
		}
		// field_type 白名单
		if _, ok := allowedFilterFieldTypes[ff.FieldType]; !ok {
			return invalidParam(fmt.Sprintf("%s.filter_fields[%d]: field_type %q not allowed (only long/tag)", path, fi, ff.FieldType))
		}
		// query_type 白名单
		if _, ok := allowedFilterQueryTypes[ff.QueryType]; !ok {
			return invalidParam(fmt.Sprintf("%s.filter_fields[%d]: query_type %q not allowed (only eq/not_eq/in/not_in)", path, fi, ff.QueryType))
		}
		// field_name 白名单: item_id 显式放行；其余仅当 field_type=tag 时视为 tag key 放行
		if _, ok := allowedFilterFieldNames[ff.FieldName]; !ok {
			if ff.FieldType != "tag" {
				return invalidParam(fmt.Sprintf("%s.filter_fields[%d]: field_name %q not allowed (only item_id or tag key with field_type=tag)", path, fi, ff.FieldName))
			}
		}
		// 点选 (item_id in) 基本校验: values 非空。
		// TODO: 文档要求"点选 values 必须全部属于对应 eval_set_version 快照(缺一报错)"，依赖 Data 侧按 version 拉 item 接口，
		// 接口就绪后在 application 层补强校验；当前降级为非空 + 数量上限。
		if ff.FieldName == "item_id" && (ff.QueryType == "in" || ff.QueryType == "not_in") {
			if len(ff.Values) == 0 {
				return invalidParam(fmt.Sprintf("%s.filter_fields[%d]: item_id %s requires non-empty values", path, fi, ff.QueryType))
			}
		}
	}
	return nil
}

// validateAlias 校验 alias 字符集与长度；空串 (默认实例) 放行。
func validateAlias(alias, path string) error {
	if alias == "" {
		return nil
	}
	if len(alias) > maxAliasLen {
		return invalidParam(fmt.Sprintf("%s: alias length exceeds %d", path, maxAliasLen))
	}
	if !aliasPattern.MatchString(alias) {
		return invalidParam(fmt.Sprintf("%s: alias %q contains invalid chars (allowed [a-zA-Z0-9_-])", path, alias))
	}
	return nil
}

func invalidParam(msg string) error {
	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(msg))
}
