// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// GetAllMetricDefinitions 获取所有指标定义
// 此函数已被移除，请使用 service/metrics 包中的 GetAllMetricDefinitions 函数
// 为了避免循环导入，请直接从新位置导入使用
func GetAllMetricDefinitions() []IMetricDefinition {
	// 返回空切片，提示用户使用新的位置
	return []IMetricDefinition{}
}