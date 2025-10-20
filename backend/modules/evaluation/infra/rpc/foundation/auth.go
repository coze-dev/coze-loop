// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package foundation

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type AuthRPCAdapter struct {
	client authservice.Client
}

func NewAuthRPCProvider(client authservice.Client) rpc.IAuthProvider {
	return &AuthRPCAdapter{
		client: client,
	}
}

type checkPermissionParam struct {
	objectID      string
	spaceID       int64
	actionObjects []*rpc.ActionObject

	withoutSPI      *bool // 评测集接口鉴权不回调SPI，直接将space_id和owner_id传递给权限接口
	OwnerID         *string
	ResourceSpaceID int64
}

func (a AuthRPCAdapter) Authorization(ctx context.Context, param *rpc.AuthorizationParam) (err error) {
	cp := make([]*checkPermissionParam, 0)
	cp = append(cp, &checkPermissionParam{
		objectID:      param.ObjectID,
		spaceID:       param.SpaceID,
		actionObjects: param.ActionObjects,
	})
	return a.checkPermission(ctx, param.SpaceID, cp)
}

func (a AuthRPCAdapter) AuthorizationWithoutSPI(ctx context.Context, param *rpc.AuthorizationWithoutSPIParam) (err error) {
	cp := make([]*checkPermissionParam, 0)
	cp = append(cp, &checkPermissionParam{
		objectID:        param.ObjectID,
		spaceID:         param.SpaceID,
		actionObjects:   param.ActionObjects,
		withoutSPI:      gptr.Of(true),
		OwnerID:         param.OwnerID,
		ResourceSpaceID: param.ResourceSpaceID,
	})
	return a.checkPermission(ctx, param.SpaceID, cp)
}

func (a AuthRPCAdapter) MAuthorizeWithoutSPI(ctx context.Context, spaceID int64, params []*rpc.AuthorizationWithoutSPIParam) error {
	cp := make([]*checkPermissionParam, 0)
	for _, param := range params {
		cp = append(cp, &checkPermissionParam{
			objectID:        param.ObjectID,
			spaceID:         param.SpaceID,
			actionObjects:   param.ActionObjects,
			withoutSPI:      gptr.Of(true),
			OwnerID:         param.OwnerID,
			ResourceSpaceID: param.ResourceSpaceID,
		})
	}

	return a.checkPermission(ctx, spaceID, cp)
}

func (a AuthRPCAdapter) checkPermission(ctx context.Context, spaceID int64, params []*checkPermissionParam) error {
	if len(params) == 0 {
		return nil
	}
	auths := make([]*authentity.SubjectActionObjects, 0)
	for _, param := range params {
		for _, actionObject := range param.actionObjects {
			auths = append(auths, &authentity.SubjectActionObjects{
				Subject: &authentity.AuthPrincipal{
					AuthPrincipalType: gptr.Of(authentity.AuthPrincipalType_CozeIdentifier),
					AuthCozeIdentifier: &authentity.AuthCozeIdentifier{
						IdentityTicket: nil,
					},
				},
				Action: actionObject.Action,
				Objects: []*authentity.AuthEntity{
					{
						ID:          &param.objectID,
						EntityType:  actionObject.EntityType,
						SpaceID:     gcond.If(gptr.Indirect(param.withoutSPI), gptr.Of(strconv.FormatInt(param.ResourceSpaceID, 10)), nil),
						OwnerUserID: gcond.If(gptr.Indirect(param.withoutSPI), param.OwnerID, nil),
					},
				},
			})
		}
	}
	mCheckReq := &auth.MCheckPermissionRequest{
		Auths:   auths,
		SpaceID: gptr.Of(spaceID),
	}
	resp, err := a.client.MCheckPermission(ctx, mCheckReq)
	if err != nil {
		return err
	}
	if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	// 有任意一个 Action 的无权限则认为无权限
	for _, r := range resp.AuthRes {
		if r == nil {
			continue
		}
		if !gptr.Indirect(r.IsAllowed) {
			logs.CtxError(ctx, "no perimission info=%v", json.Jsonify(r))
			return errorx.NewByCode(errno.CommonNoPermissionCode)
		}
	}
	return nil
}
