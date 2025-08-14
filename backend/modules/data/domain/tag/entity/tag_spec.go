// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type TagSpec struct {
	// 最大层数
	MaxHeight int `json:"max_height" mapstructure:"max_height"`
	// 每层最大宽度
	MaxWidth int `json:"max_width" mapstructure:"max_width"`
}
