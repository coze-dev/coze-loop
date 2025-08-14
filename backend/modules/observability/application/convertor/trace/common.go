// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func UserInfoDO2DTO(info *commonentity.UserInfo) *commondto.UserInfo {
	if info == nil {
		return nil
	}
	ret := &commondto.UserInfo{}
	if info.Name != "" {
		ret.Name = ptr.Of(info.Name)
	}
	if info.EnName != "" {
		ret.EnName = ptr.Of(info.EnName)
	}
	if info.AvatarURL != "" {
		ret.AvatarURL = ptr.Of(info.AvatarURL)
	}
	if info.AvatarThumb != "" {
		ret.AvatarThumb = ptr.Of(info.AvatarThumb)
	}
	if info.OpenID != "" {
		ret.OpenID = ptr.Of(info.OpenID)
	}
	if info.UnionID != "" {
		ret.UnionID = ptr.Of(info.UnionID)
	}
	if info.Email != "" {
		ret.Email = ptr.Of(info.Email)
	}
	if info.UserID != "" {
		ret.UserID = ptr.Of(info.UserID)
	}
	return ret
}
