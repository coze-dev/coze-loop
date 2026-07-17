// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 飞书 receive_id_type 取值，对齐 Lark OpenAPI。
const (
	receiveIDTypeEmail   = "email"
	receiveIDTypeOpenID  = "open_id"
	receiveIDTypeUnionID = "union_id"
)

// invalidNotifyUserID 无法作为通知目标的占位/非法 user_id。
// THEMIS_END_UID_INVALID：Service Account JWT 无对应真实终端用户时的占位值；
// "0"：空 user id 的数值化占位。
func invalidNotifyUserID(userID string) bool {
	return userID == "" || userID == "THEMIS_END_UID_INVALID" || userID == "0"
}

// resolveNotifyTarget 解析飞书通知目标。
//
// 取值优先级：NotificationConf.FeishuNotification.UserID > expt.CreatedBy。
// 再按 user_id 的格式推断飞书 receive_id_type：
//   - 含 "@"        → email，直接作为接收方
//   - "ou_" 前缀    → open_id
//   - "on_" 前缀    → union_id
//   - 纯数字/其它    → 视为 Fornax 平台 user ID，走 MGetUserInfo 查 email 后以 email 发送
//
// 无法解析出有效目标时返回 ("", "")，调用方据此静默跳过（不发通知）。
func resolveNotifyTarget(ctx context.Context, userProvider rpc.IUserProvider, expt *entity.Experiment) (receiveID string, receiveIDType string) {
	userID := ""
	if expt.NotificationConf != nil &&
		expt.NotificationConf.FeishuNotification != nil &&
		gptr.Indirect(expt.NotificationConf.FeishuNotification.UserID) != "" {
		userID = gptr.Indirect(expt.NotificationConf.FeishuNotification.UserID)
	}
	if userID == "" {
		userID = expt.CreatedBy
	}
	if invalidNotifyUserID(userID) {
		return "", ""
	}

	switch {
	case strings.Contains(userID, "@"):
		return userID, receiveIDTypeEmail
	case strings.HasPrefix(userID, "ou_"):
		return userID, receiveIDTypeOpenID
	case strings.HasPrefix(userID, "on_"):
		return userID, receiveIDTypeUnionID
	default:
		// Fornax 平台 user ID：查 MGetUserInfo 拿 email，以 email 发送。
		userInfos, err := userProvider.MGetUserInfo(ctx, []string{userID})
		if err != nil || len(userInfos) == 0 || userInfos[0] == nil || gptr.Indirect(userInfos[0].Email) == "" {
			return "", ""
		}
		return gptr.Indirect(userInfos[0].Email), receiveIDTypeEmail
	}
}
