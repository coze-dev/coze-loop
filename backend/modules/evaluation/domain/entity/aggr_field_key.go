// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"strings"
)

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
