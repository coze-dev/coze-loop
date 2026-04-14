// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func SessionMW(ss session.ISessionService, us userservice.Client) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Path())
		if path == "/api/foundation/v1/users/login_by_password" ||
			path == "/api/foundation/v1/users/register" {
			c.Next(ctx)
			return
		}

		sess, err := ss.ValidateSession(ctx, string(c.Cookie(session.SessionKey)))
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		resp, err := us.GetUserInfo(ctx, &user.GetUserInfoRequest{
			UserID: ptr.Of(sess.UserID),
		})
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		if resp.GetUserInfo() == nil {
			_ = c.Error(fmt.Errorf("invalid session user, user_id %s not found", sess.UserID))
			c.Abort()
			return
		}

		ctx = session.WithCtxUser(ctx, &session.User{
			ID:    sess.UserID,
			Name:  resp.GetUserInfo().GetName(),
			Email: resp.GetUserInfo().GetEmail(),
		})

		c.Next(ctx)
	}
}
