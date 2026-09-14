// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/stone/fornax/ml_flow/domain/filter"
)

// filterFieldItemID 是唯一被白名单放行、可转为 item_id 过滤的筛选字段名。
// 其它 field_name 一律忽略,绝不拼成任意列过滤。
const filterFieldItemID = "item_id"

// parseItemIDFilter 从通用 Filter 中解析出面向 item_id 的精确筛选值集合。
// 只识别 field_name == item_id 的 FilterField,其它字段直接忽略。
// query_type 为 eq / in 均落成 item_id 值集合(DAL 走 item_id IN(...));
// 其它 query_type 按 in 处理(取全部 values),语义仍是 item_id 属于该集合。
// values 为字符串,item_id 底层是 i64,解析失败的值跳过而非报错。
func parseItemIDFilter(f *filter.Filter) []int64 {
	if f == nil || len(f.FilterFields) == 0 {
		return nil
	}
	var itemIDs []int64
	for _, field := range f.FilterFields {
		if field == nil || field.GetFieldName() != filterFieldItemID {
			continue
		}
		for _, v := range field.GetValues() {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				continue
			}
			itemIDs = append(itemIDs, id)
		}
	}
	return itemIDs
}
