// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import "math"

// RoundScoreToTwoDecimals 将分数四舍五入到两位小数
func RoundScoreToTwoDecimals(score float64) float64 {
	multiplier := 100.0
	return math.Round(score*multiplier) / multiplier
}

// RoundScorePtrToTwoDecimals 将分数指针四舍五入到两位小数，如果为nil则返回nil
func RoundScorePtrToTwoDecimals(score *float64) *float64 {
	if score == nil {
		return nil
	}
	rounded := RoundScoreToTwoDecimals(*score)
	return &rounded
}
