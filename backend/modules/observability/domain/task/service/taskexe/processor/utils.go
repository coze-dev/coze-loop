// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func getSession(ctx context.Context, task *task.Task) *common.Session {
	userIDStr := session.UserIDInCtxOrEmpty(ctx)
	if userIDStr == "" {
		userIDStr = task.GetBaseInfo().GetCreatedBy().GetUserID()
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] AutoEvaluteProcessor OnChangeProcessor, ParseInt err:%v", err)
	}
	return &common.Session{
		UserID: gptr.Of(userID),
		//AppID:  gptr.Of(int32(717152)),
	}
}
