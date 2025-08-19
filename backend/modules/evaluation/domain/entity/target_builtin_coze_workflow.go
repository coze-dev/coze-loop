// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type CozeWorkflow struct {
	ID      string
	Version string
	EndType int32 // 结束节点的类型，1：返回文本

	Name        string    `json:"-"`
	AvatarURL   string    `json:"-"`
	Description string    `json:"-"`
	BaseInfo    *BaseInfo `json:"-"`
}
