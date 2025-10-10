// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package ck

import "github.com/samber/lo"

func getColumnStr(columns []string, omits []string) string {
	omitMap := lo.Associate(omits, func(omit string) (string, bool) {
		return omit, true
	})
	result := ""
	for _, c := range columns {
		if omitMap[c] {
			continue
		}
		if result != "" {
			result += ", "
		}
		result += c
	}
	return result
}
