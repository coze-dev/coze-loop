// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"strconv"
	"time"

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

// shouldTriggerBackfill 判断是否需要发送历史回溯MQ
func ShouldTriggerBackfill(taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}

	// 检查回填时间配置
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	backfillTime := rule.GetBackfillEffectiveTime()
	if backfillTime == nil {
		return false
	}

	return backfillTime.GetStartAt() > 0 &&
		backfillTime.GetEndAt() > 0 &&
		backfillTime.GetStartAt() < backfillTime.GetEndAt()
}

func ShouldTriggerNewData(ctx context.Context, taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	effectiveTime := rule.GetEffectiveTime()
	if effectiveTime == nil {
		return false
	}
	logs.CtxInfo(ctx, "[auto_task] ShouldTriggerNewData, endAt:%d, startAt:%d", effectiveTime.GetEndAt(), effectiveTime.GetStartAt())

	return effectiveTime.GetEndAt() > 0 &&
		effectiveTime.GetStartAt() > 0 &&
		effectiveTime.GetStartAt() < effectiveTime.GetEndAt() &&
		time.Now().After(time.UnixMilli(effectiveTime.GetStartAt()))
}
