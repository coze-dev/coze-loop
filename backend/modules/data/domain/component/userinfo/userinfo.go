// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package userinfo

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/common"
	common2 "github.com/coze-dev/coze-loop/backend/modules/data/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	evalerr "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type UserInfoCarrier interface {
	GetBaseInfo() *common.BaseInfo
	SetBaseInfo(*common.BaseInfo)
}

type UserInfoDomainCarrier interface {
	GetBaseInfo() *entity.BaseInfo
	SetBaseInfo(*entity.BaseInfo)
}

//go:generate mockgen -destination=mocks/userinfo.go -package=mocks . UserInfoService
type UserInfoService interface {
	PackUserInfo(ctx context.Context, userInfoCarrier interface{})
}

type UserInfoServiceImpl struct {
	userProvider rpc.IUserProvider
}

func NewUserInfoServiceImpl(userProvider rpc.IUserProvider) UserInfoService {
	return &UserInfoServiceImpl{
		userProvider: userProvider,
	}
}

func (u *UserInfoServiceImpl) GetUserInfo(ctx context.Context, userID string) (*common.UserInfo, error) {
	infos, err := u.userProvider.MGetUserInfo(ctx, []string{userID})
	if err != nil {
		return nil, errorx.WrapByCode(err, evalerr.CommonRPCErrorCode)
	}

	if len(infos) == 0 {
		return nil, errorx.WrapByCode(err, evalerr.CommonRPCErrorCode)
	}

	return common2.ConvertUserInfoDO2DTO(infos[0]), nil
}

func (u *UserInfoServiceImpl) PackUserInfo(ctx context.Context, userInfoCarriers interface{}) {
	var carriers []interface{}
	var getBaseInfoFunc func(interface{}) interface{}
	var setCreatedByFunc func(interface{}, interface{})
	var setUpdatedByFunc func(interface{}, interface{})
	var getCreatedByFunc func(interface{}) string
	var getUpdatedByFunc func(interface{}) string
	var convertInfoFunc func(interface{}) interface{}

	switch carriersType := userInfoCarriers.(type) {
	case []UserInfoCarrier:
		carriers = make([]interface{}, 0, len(carriersType))
		for _, dto := range carriersType {
			carriers = append(carriers, dto)
		}
		getBaseInfoFunc = func(c interface{}) interface{} {
			return c.(UserInfoCarrier).GetBaseInfo()
		}
		setCreatedByFunc = func(baseInfo, info interface{}) {
			baseInfo.(*common.BaseInfo).SetCreatedBy(info.(*common.UserInfo))
		}
		setUpdatedByFunc = func(baseInfo, info interface{}) {
			baseInfo.(*common.BaseInfo).SetUpdatedBy(info.(*common.UserInfo))
		}
		getCreatedByFunc = func(c interface{}) string {
			return c.(UserInfoCarrier).GetBaseInfo().GetCreatedBy().GetUserID()
		}
		getUpdatedByFunc = func(c interface{}) string {
			return c.(UserInfoCarrier).GetBaseInfo().GetUpdatedBy().GetUserID()
		}
		convertInfoFunc = func(info interface{}) interface{} {
			return common2.ConvertUserInfoDO2DTO(info.(*entity.UserInfo))
		}
	case []UserInfoDomainCarrier:
		carriers = make([]interface{}, 0, len(carriersType))
		for _, dto := range carriersType {
			carriers = append(carriers, dto)
		}
		getBaseInfoFunc = func(c interface{}) interface{} {
			return c.(UserInfoDomainCarrier).GetBaseInfo()
		}
		setCreatedByFunc = func(baseInfo, info interface{}) {
			baseInfo.(*entity.BaseInfo).SetCreatedBy(info.(*entity.UserInfo))
		}
		setUpdatedByFunc = func(baseInfo, info interface{}) {
			baseInfo.(*entity.BaseInfo).SetUpdatedBy(info.(*entity.UserInfo))
		}
		getCreatedByFunc = func(c interface{}) string {
			return gptr.Indirect(c.(UserInfoDomainCarrier).GetBaseInfo().GetCreatedBy().UserID)
		}
		getUpdatedByFunc = func(c interface{}) string {
			return gptr.Indirect(c.(UserInfoDomainCarrier).GetBaseInfo().GetUpdatedBy().UserID)
		}
		convertInfoFunc = func(info interface{}) interface{} {
			return info.(*entity.UserInfo)
		}
	default:
		return
	}

	if len(carriers) == 0 {
		return
	}

	userIDs := make(map[string]struct{})
	for _, userInfoCarrier := range carriers {
		if userInfoCarrier == nil {
			continue
		}
		if createdBy := getCreatedByFunc(userInfoCarrier); len(createdBy) > 0 {
			userIDs[createdBy] = struct{}{}
		}
		if updatedBy := getUpdatedByFunc(userInfoCarrier); len(updatedBy) > 0 {
			userIDs[updatedBy] = struct{}{}
		}
	}

	userIDList := make([]string, 0, len(userIDs))
	for userID := range userIDs {
		userIDList = append(userIDList, userID)
	}

	infos, err := u.userProvider.MGetUserInfo(ctx, userIDList)
	if err != nil {
		// 忽略获取用户信息时的错误
		logs.CtxError(ctx, "get user info failed: %v, userIDs=%v", err, json.Jsonify(userIDList))
		return
	}

	infoMap := make(map[string]interface{})
	for _, info := range infos {
		if info == nil {
			continue
		}
		infoMap[gptr.Indirect(info.UserID)] = convertInfoFunc(info)
	}

	for _, userInfoCarrier := range carriers {
		if userInfoCarrier == nil {
			continue
		}
		baseInfo := getBaseInfoFunc(userInfoCarrier)
		if createdBy := getCreatedByFunc(userInfoCarrier); len(createdBy) > 0 {
			if info, ok := infoMap[createdBy]; ok {
				setCreatedByFunc(baseInfo, info)
			}
		}
		if updatedBy := getUpdatedByFunc(userInfoCarrier); len(updatedBy) > 0 {
			if info, ok := infoMap[updatedBy]; ok {
				setUpdatedByFunc(baseInfo, info)
			}
		}
	}
}

func BatchConvertDTO2UserInfoCarrier[T UserInfoCarrier](dto []T) []UserInfoCarrier {
	if len(dto) == 0 {
		return nil
	}
	carriers := make([]UserInfoCarrier, 0, len(dto))
	for _, d := range dto {
		carriers = append(carriers, d)
	}
	return carriers
}

func BatchConvertDO2UserInfoDomainCarrier[T UserInfoDomainCarrier](do []T) []UserInfoDomainCarrier {
	if len(do) == 0 {
		return nil
	}
	carriers := make([]UserInfoDomainCarrier, 0, len(do))
	for _, d := range do {
		carriers = append(carriers, d)
	}
	return carriers
}
