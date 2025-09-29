// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type AuthRPCAdapter struct {
	client authservice.Client
}

func NewAuthRPCProvider(client authservice.Client) rpc.IAuthProvider {
	return &AuthRPCAdapter{
		client: client,
	}
}

func (a *AuthRPCAdapter) mCheckPromptPermissionBase(ctx context.Context, spaceID int64, promptIDs []int64, action string) error {
	var authPairs []*authentity.SubjectActionObjects
	authSubject := &authentity.AuthPrincipal{
		AuthPrincipalType:  authentity.AuthPrincipalTypePtr(authentity.AuthPrincipalType_CozeIdentifier),
		AuthCozeIdentifier: &authentity.AuthCozeIdentifier{IdentityTicket: nil},
	}
	for _, promptID := range promptIDs {
		authPairs = append(authPairs, &authentity.SubjectActionObjects{
			Subject: authSubject,
			Action:  ptr.Of(action),
			Objects: []*authentity.AuthEntity{
				{
					ID:         ptr.Of(fmt.Sprint(promptID)),
					EntityType: ptr.Of(authentity.AuthEntityTypePrompt),
				},
			},
		})
	}
	req := &auth.MCheckPermissionRequest{
		Auths:   authPairs,
		SpaceID: ptr.Of(spaceID),
	}
	resp, err := a.client.MCheckPermission(ctx, req)
	if err != nil {
		return err
	}
	var reject bool
	var rejectedMsgs []string
	for _, authRes := range resp.AuthRes {
		if authRes != nil && !authRes.GetIsAllowed() {
			reject = true
			rejectedPromptIDs := make([]string, 0)
			if objects := authRes.SubjectActionObjects; objects != nil {
				for _, object := range objects.Objects {
					rejectedPromptIDs = append(rejectedPromptIDs, ptr.From(object.ID))
				}
			}
			rejectedMsgs = append(rejectedMsgs, fmt.Sprintf("prompt ids: %s, action: %s", strings.Join(rejectedPromptIDs, ","), action))
		}
	}
	if reject {
		errMsg := strings.Join(rejectedMsgs, ";")
		return errorx.NewByCode(prompterr.CommonNoPermissionCode, errorx.WithExtraMsg(errMsg))
	}
	return nil
}

func (a *AuthRPCAdapter) MCheckPromptPermission(ctx context.Context, spaceID int64, promptIDs []int64, action string) error {
	return a.mCheckPromptPermissionBase(ctx, spaceID, promptIDs, action)
}

func (a *AuthRPCAdapter) MCheckPromptPermissionForOpenAPI(ctx context.Context, spaceID int64, promptIDs []int64, action string) error {
	return a.mCheckPromptPermissionBase(ctx, spaceID, promptIDs, action)
}

func (a *AuthRPCAdapter) checkSpacePermissionBase(ctx context.Context, spaceID int64, action string) error {
	authSubject := &authentity.AuthPrincipal{
		AuthPrincipalType:  authentity.AuthPrincipalTypePtr(authentity.AuthPrincipalType_CozeIdentifier),
		AuthCozeIdentifier: &authentity.AuthCozeIdentifier{IdentityTicket: nil},
	}
	authPairs := []*authentity.SubjectActionObjects{
		{
			Subject: authSubject,
			Action:  ptr.Of(action),
			Objects: []*authentity.AuthEntity{
				{
					ID:         ptr.Of(fmt.Sprint(spaceID)),
					EntityType: ptr.Of(authentity.AuthEntityTypeSpace),
				},
			},
		},
	}
	req := &auth.MCheckPermissionRequest{
		Auths:   authPairs,
		SpaceID: ptr.Of(spaceID),
	}
	resp, err := a.client.MCheckPermission(ctx, req)
	if err != nil {
		return err
	}
	for _, authRes := range resp.AuthRes {
		if authRes != nil && !authRes.GetIsAllowed() {
			return errorx.NewByCode(prompterr.CommonNoPermissionCode)
		}
	}
	return nil
}

func (a *AuthRPCAdapter) CheckSpacePermission(ctx context.Context, spaceID int64, action string) error {
	return a.checkSpacePermissionBase(ctx, spaceID, action)
}

func (a *AuthRPCAdapter) CheckSpacePermissionForOpenAPI(ctx context.Context, spaceID int64, action string) error {
	return a.checkSpacePermissionBase(ctx, spaceID, action)
}
