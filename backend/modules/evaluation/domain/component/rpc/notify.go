// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

//go:generate mockgen -destination=mocks/notify.go -package=mocks . INotifyRPCAdapter
type INotifyRPCAdapter interface {
	// SendMessageCard 发送飞书卡片消息。
	// receiveID 为接收方标识，receiveIDType 为其类型（email / open_id / union_id 等，对应飞书 receive_id_type）。
	SendMessageCard(ctx context.Context, receiveID, receiveIDType, cardID string, param map[string]string) error
}
