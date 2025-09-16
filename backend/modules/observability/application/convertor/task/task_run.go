// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	entity_common "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func TaskRunPOs2DOs(ctx context.Context, taskRunPOs []*entity.TaskRun, userInfos map[string]*entity_common.UserInfo) []*task.TaskRun {
	var taskRunList []*task.TaskRun
	if len(taskRunPOs) == 0 {
		return taskRunList
	}
	for _, v := range taskRunPOs {
		taskRunDO := TaskRunPO2DTO(ctx, v, userInfos)
		taskRunList = append(taskRunList, taskRunDO)
	}
	return taskRunList
}
func TaskRunPO2DTO(ctx context.Context, v *entity.TaskRun, userMap map[string]*entity_common.UserInfo) *task.TaskRun {
	if v == nil {
		return nil
	}
	taskRunInfo := &task.TaskRun{
		ID:                v.ID,
		WorkspaceID:       v.WorkspaceID,
		TaskID:            v.TaskID,
		TaskType:          v.TaskType,
		RunStatus:         v.RunStatus,
		RunDetail:         RunDetailPO2DTO(ctx, v.RunDetail),
		BackfillRunDetail: BackfillRunDetailPO2DTO(ctx, v.BackfillDetail),
		RunStartAt:        v.RunStartAt.UnixMilli(),
		RunEndAt:          v.RunEndAt.UnixMilli(),
		TaskRunConfig:     TaskRunConfigPO2DTO(ctx, v.RunConfig),
		BaseInfo:          buildTaskRunBaseInfo(v, userMap),
	}
	return taskRunInfo
}

// RunDetailPO2DTO 将JSON字符串转换为RunDetail结构体
func RunDetailPO2DTO(ctx context.Context, runDetail *string) *task.RunDetail {
	if runDetail == nil || *runDetail == "" {
		return nil
	}

	var runDetailDTO task.RunDetail
	if err := sonic.Unmarshal([]byte(*runDetail), &runDetailDTO); err != nil {
		logs.CtxError(ctx, "RunDetailPO2DTO sonic.Unmarshal err:%v", err)
		return nil
	}

	return &runDetailDTO
}

// RunDetailPO2DTO 将JSON字符串转换为RunDetail结构体
func BackfillRunDetailPO2DTO(ctx context.Context, runDetail *string) *task.BackfillDetail {
	if runDetail == nil || *runDetail == "" {
		return nil
	}

	var runDetailDTO task.BackfillDetail
	if err := sonic.Unmarshal([]byte(*runDetail), &runDetailDTO); err != nil {
		logs.CtxError(ctx, "RunDetailPO2DTO sonic.Unmarshal err:%v", err)
		return nil
	}

	return &runDetailDTO
}

// TaskRunConfigPO2DTO 将JSON字符串转换为TaskRunConfig结构体
func TaskRunConfigPO2DTO(ctx context.Context, runConfig *string) *task.TaskRunConfig {
	if runConfig == nil || *runConfig == "" {
		return nil
	}

	var runConfigDTO task.TaskRunConfig
	if err := sonic.Unmarshal([]byte(*runConfig), &runConfigDTO); err != nil {
		logs.CtxError(ctx, "TaskRunConfigPO2DTO sonic.Unmarshal err:%v", err)
		return nil
	}

	return &runConfigDTO
}

// buildTaskRunBaseInfo 构建BaseInfo信息
func buildTaskRunBaseInfo(v *entity.TaskRun, userMap map[string]*entity_common.UserInfo) *common.BaseInfo {
	// 注意：TaskRun实体中没有CreatedBy和UpdatedBy字段
	// 使用空字符串作为默认值
	return &common.BaseInfo{
		CreatedAt: gptr.Of(v.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(v.UpdatedAt.UnixMilli()),
		CreatedBy: &common.UserInfo{UserID: gptr.Of("")},
		UpdatedBy: &common.UserInfo{UserID: gptr.Of("")},
	}
}
