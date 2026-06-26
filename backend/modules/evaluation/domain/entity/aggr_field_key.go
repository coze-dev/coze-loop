// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"strings"
)

// EncodeEvaluatorInstanceKey 把评估器实例编码为统一的 string key, 用于按 (versionID, alias)
// 索引加权计算相关的 map, 治理同 versionID 多别名(alias)撞 key 的问题.
//
// 编码规则 (与 ParseEvaluatorScoreFieldKey 互逆):
//   - alias == "": 退化为裸 "<versionID>" (不带冒号), 保证旧数据/旧实验 byte 级不变
//   - alias != "": "<versionID>:<alias>"
func EncodeEvaluatorInstanceKey(versionID int64, alias string) string {
	if alias == "" {
		return strconv.FormatInt(versionID, 10)
	}
	return strconv.FormatInt(versionID, 10) + ":" + alias
}

// ParseEvaluatorScoreFieldKey 解析 expt_aggr_result.field_key (field_type=EvaluatorScore).
//
// 兼容两种格式:
//   - 纯数字 "<versionID>": 当前格式 (单实例,无 alias), 返回 alias=""
//   - "<versionID>:<alias>": 未来扩展格式 (alias 多实例); 当前生产代码不写入,
//     但读侧解析提前兼容,避免后续若启用 alias field_key 时崩溃
//
// 解析失败 (非数字 versionID) 返回 err。
func ParseEvaluatorScoreFieldKey(fk string) (versionID int64, alias string, err error) {
	if i := strings.IndexByte(fk, ':'); i >= 0 {
		versionID, err = strconv.ParseInt(fk[:i], 10, 64)
		if err != nil {
			return 0, "", err
		}
		alias = fk[i+1:]
		return
	}
	versionID, err = strconv.ParseInt(fk, 10, 64)
	return
}
