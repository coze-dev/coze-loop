// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	authModel "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/domain/user/repo"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type AuthApplicationImpl struct {
	userRepo repo.IUserRepo
}

func NewAuthApplication(userRepo repo.IUserRepo) auth.AuthService {
	return &AuthApplicationImpl{
		userRepo: userRepo,
	}
}

func (a *AuthApplicationImpl) MCheckPermission(ctx context.Context, request *auth.MCheckPermissionRequest) (r *auth.MCheckPermissionResponse, err error) {
	logs.CtxDebug(ctx, "MCheckPermission request: %+v", request)

	userID, err := strconv.ParseInt(session.UserIDInCtxOrEmpty(ctx), 10, 64)
	if userID == 0 || err != nil {
		logs.CtxError(ctx, "invalid user_id in context: %v", userID)
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid user_id in context"))
	}

	spaceID := request.GetSpaceID()
	if spaceID <= 0 {
		logs.CtxError(ctx, "invalid space_id in request: %v", spaceID)
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid space_id in request"))
	}

	isUserSpace, err := a.userRepo.CheckUserSpaceExist(ctx, userID, spaceID)
	if err != nil {
		logs.CtxError(ctx, "failed to check user space, userID: %v, spaceID: %v, err: %v", userID, spaceID, err)
		return nil, err
	}

	authRes := make([]*authModel.SubjectActionObjectAuthRes, 0, len(request.Auths))
	for _, authObject := range request.Auths {
		isAllowed := true
		for _, object := range authObject.Objects {
			if object.SpaceID != nil && object.GetSpaceID() != strconv.FormatInt(spaceID, 10) {
				isAllowed = false
				break
			}
		}

		authRes = append(authRes, &authModel.SubjectActionObjectAuthRes{
			SubjectActionObjects: authObject,
			IsAllowed:            ptr.Of(isUserSpace && isAllowed),
		})
	}

	return &auth.MCheckPermissionResponse{
		AuthRes: authRes,
	}, nil
}
