// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"strconv"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// FilterMode 语义 (per ItemEvaluatorConf.FilterMode):
//   0 None    - 不过滤, 直接跑
//   1 Include - filter 命中才跑
//   2 Exclude - filter 命中不跑
const (
	filterModeNone    int32 = 0
	filterModeInclude int32 = 1
	filterModeExclude int32 = 2
)

// ShouldRunByFilter 综合 FilterMode + matcher 结果, 判断该 evaluator binding 是否应该实际执行。
// 返回 false 时调用方应当生成 Status=Skipped 的占位 record (供 GUI 展示 + 数仓使用)。
//
// 兜底原则: filter 异常时倾向 "跑" (避免静默漏跑), 上层可根据 err 决定是否告警。
func ShouldRunByFilter(filter *entity.ExptItemFilter, filterMode int32, item *entity.EvaluationSetItem, turn *entity.Turn) (bool, error) {
	if filterMode == filterModeNone || filter == nil || len(filter.FilterFields) == 0 {
		return true, nil
	}
	matched, err := MatchExptItemFilter(filter, item, turn)
	if err != nil {
		return true, err // 异常 → 默认跑, 同时回传 err
	}
	switch filterMode {
	case filterModeInclude:
		return matched, nil
	case filterModeExclude:
		return !matched, nil
	default:
		return true, nil
	}
}

// MatchExptItemFilter 计算 filter 是否命中给定 item/turn。
// QueryAndOr 决定多 FilterField 之间的逻辑 ("and"/空为 AND, "or" 为 OR)。
// 单个 FilterField 按 QueryType 对字段值做匹配。
func MatchExptItemFilter(filter *entity.ExptItemFilter, item *entity.EvaluationSetItem, turn *entity.Turn) (bool, error) {
	if filter == nil || len(filter.FilterFields) == 0 {
		return true, nil
	}

	useOr := strings.EqualFold(filter.QueryAndOr, "or")

	for _, ff := range filter.FilterFields {
		if ff == nil {
			continue
		}
		fieldMatched := matchFilterField(ff, item, turn)
		if useOr {
			if fieldMatched {
				return true, nil
			}
		} else {
			if !fieldMatched {
				return false, nil
			}
		}
	}

	if useOr {
		return false, nil // OR: 没有任一命中
	}
	return true, nil // AND: 全部命中 (或全部为空已 continue)
}

// matchFilterField 单字段匹配。
//   - field_name=item_id: 用 item.ItemID 比较 (item 级, 不依赖 turn)
//   - 其余: 从 turn.FieldDataList 按 name/key 取字段文本值
//
// 注: tag (field_type=tag) 暂未支持 — matcher 尚无 tag 分支, 配 tag 会走文本路径匹配不上,
// 见 tech debt; 普通 turn 字段创建侧白名单当前未放行 (仅 item_id / tag)。
func matchFilterField(ff *entity.ExptItemFilterField, item *entity.EvaluationSetItem, turn *entity.Turn) bool {
	// item_id: 直接读 item 级 ItemID, 不走 turn 文本路径
	if ff.FieldName == "item_id" {
		if item == nil {
			return matchMissingField(ff.QueryType)
		}
		return matchByQueryType(ff.QueryType, strconv.FormatInt(item.ItemID, 10), ff.Values)
	}

	actual, ok := getFieldTextValue(ff.FieldName, item, turn)
	if !ok {
		return matchMissingField(ff.QueryType)
	}
	return matchByQueryType(ff.QueryType, actual, ff.Values)
}

// matchMissingField 字段不存在时的语义: equal/in 不命中 (false); not_equal/not_in 取反命中 (true)。
func matchMissingField(queryType string) bool {
	switch strings.ToLower(queryType) {
	case "not_equal", "ne", "not_in", "not_eq":
		return true
	}
	return false
}

// getFieldTextValue 从 turn.FieldDataList 取 fieldName 对应的文本值 (匹配 Name 或 Key)。
// item 暂未使用 (item 级字段后续可扩展), 保留参数以备扩展。
func getFieldTextValue(fieldName string, _ *entity.EvaluationSetItem, turn *entity.Turn) (string, bool) {
	if turn == nil {
		return "", false
	}
	for _, fd := range turn.FieldDataList {
		if fd == nil {
			continue
		}
		if fd.Name == fieldName || fd.Key == fieldName {
			if fd.Content != nil && fd.Content.Text != nil {
				return *fd.Content.Text, true
			}
			return "", true // 字段存在但 Text 为 nil
		}
	}
	return "", false
}

// matchByQueryType 按 QueryType 比较 actual 和 values。空 QueryType 默认按 equal/in 处理。
func matchByQueryType(queryType, actual string, values []string) bool {
	switch strings.ToLower(queryType) {
	case "", "equal", "eq", "=", "in":
		// 任一值相等 → 命中
		for _, v := range values {
			if v == actual {
				return true
			}
		}
		return false
	case "not_equal", "ne", "not_in", "not_eq":
		for _, v := range values {
			if v == actual {
				return false
			}
		}
		return true
	case "contains":
		for _, v := range values {
			if strings.Contains(actual, v) {
				return true
			}
		}
		return false
	case "not_contains":
		for _, v := range values {
			if strings.Contains(actual, v) {
				return false
			}
		}
		return true
	default:
		// 未识别的 QueryType 默认放行 (不阻断执行)
		return true
	}
}

