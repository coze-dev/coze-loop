// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"reflect"
	"strconv"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NewRequestSessionMW 创建处理request参数中session的middleware
// 该middleware通过反射检测request参数中是否包含session字段（类型为*common.Session）
// 如果存在，则提取用户信息并使用WithCtxUser函数注入到context中
func NewRequestSessionMW() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			// 尝试从request参数中提取session信息
			if session := extractSessionFromRequest(req); session != nil {
				// 构造User对象并注入到context中
				user := &User{
					ID:    strconv.FormatInt(session.GetUserID(), 10), // i64转string
					AppID: session.GetAppID(),                         // i32
					// Name和Email暂时为空，可根据需要从其他地方获取
				}
				ctx = WithCtxUser(ctx, user)
				logs.CtxDebug(ctx, "RequestSessionMW: injected user to context, userID=%s, appID=%d", user.ID, user.AppID)
			}

			return next(ctx, req, resp)
		}
	}
}

// extractSessionFromRequest 使用反射从request参数中提取session字段
// 支持*common.Session类型的session字段
func extractSessionFromRequest(req any) *common.Session {
	if req == nil {
		return nil
	}

	val := reflect.ValueOf(req)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	// 查找名为"Session"的字段
	sessionField := val.FieldByName("Session")
	if !sessionField.IsValid() {
		return nil
	}

	// 检查字段是否为nil
	if sessionField.IsNil() {
		return nil
	}

	// 尝试类型断言为*common.Session
	if session, ok := sessionField.Interface().(*common.Session); ok {
		return session
	}

	return nil
}
