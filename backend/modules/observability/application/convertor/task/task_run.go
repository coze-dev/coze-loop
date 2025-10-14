// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

//	func TaskRunPOs2DOs(ctx context.Context, taskRunPOs []*entity.TaskRun, userInfos map[string]*entity_common.UserInfo) []*task.TaskRun {
//		var taskRunList []*task.TaskRun
//		if len(taskRunPOs) == 0 {
//			return taskRunList
//		}
//		for _, v := range taskRunPOs {
//			taskRunDO := TaskRunPO2DTO(ctx, v, userInfos)
//			taskRunList = append(taskRunList, taskRunDO)
//		}
//		return taskRunList
//	}
//
//	func TaskRunPO2DTO(ctx context.Context, v *entity.TaskRun, userMap map[string]*entity_common.UserInfo) *task.TaskRun {
//		if v == nil {
//			return nil
//		}
//		taskRunInfo := &task.TaskRun{
//			ID:                v.ID,
//			WorkspaceID:       v.WorkspaceID,
//			TaskID:            v.TaskID,
//			TaskType:          v.TaskType,
//			RunStatus:         v.RunStatus,
//			RunDetail:         RunDetailPO2DTO(ctx, v.RunDetail),
//			BackfillRunDetail: BackfillRunDetailPO2DTO(ctx, v.BackfillDetail),
//			RunStartAt:        v.RunStartAt.UnixMilli(),
//			RunEndAt:          v.RunEndAt.UnixMilli(),
//			TaskRunConfig:     TaskRunConfigPO2DTO(ctx, v.RunConfig),
//			BaseInfo:          buildTaskRunBaseInfo(v, userMap),
//		}
//		return taskRunInfo
//	}
//func TaskRunDO2PO(ctx context.Context, v *task.TaskRun, userMap map[string]*entity_common.UserInfo) *entity.TaskRun {
//	if v == nil {
//		return nil
//	}
//	taskRunPO := &entity.TaskRun{
//		ID:             v.ID,
//		WorkspaceID:    v.WorkspaceID,
//		TaskID:         v.TaskID,
//		TaskType:       v.TaskType,
//		RunStatus:      v.RunStatus,
//		RunDetail:      RunDetailDTO2PO(ctx, v.RunDetail),
//		BackfillDetail: BackfillRunDetailDTO2PO(ctx, v.BackfillRunDetail),
//		RunStartAt:     time.UnixMilli(v.RunStartAt),
//		RunEndAt:       time.UnixMilli(v.RunEndAt),
//		RunConfig:      TaskRunConfigDTO2PO(ctx, v.TaskRunConfig),
//	}
//	return taskRunPO
//}
//
//func RunDetailDTO2PO(ctx context.Context, v *task.RunDetail) *string {
//	if v == nil {
//		return nil
//	}
//	runDetailJSON, err := sonic.MarshalString(v)
//	if err != nil {
//		logs.CtxError(ctx, "RunDetailDTO2PO sonic.MarshalString err:%v", err)
//		return nil
//	}
//	return gptr.Of(runDetailJSON)
//}
//
//func BackfillRunDetailDTO2PO(ctx context.Context, v *task.BackfillDetail) *string {
//	if v == nil {
//		return nil
//	}
//	backfillDetailJSON, err := sonic.MarshalString(v)
//	if err != nil {
//		logs.CtxError(ctx, "BackfillRunDetailDTO2PO sonic.MarshalString err:%v", err)
//		return nil
//	}
//	return gptr.Of(backfillDetailJSON)
//}
//
//func TaskRunConfigDTO2PO(ctx context.Context, v *task.TaskRunConfig) *string {
//	if v == nil {
//		return nil
//	}
//	taskRunConfigJSON, err := sonic.MarshalString(v)
//	if err != nil {
//		logs.CtxError(ctx, "TaskRunConfigDTO2PO sonic.MarshalString err:%v", err)
//		return nil
//	}
//	return gptr.Of(taskRunConfigJSON)
//}
//
//// RunDetailPO2DTO 将JSON字符串转换为RunDetail结构体
//func RunDetailPO2DTO(ctx context.Context, runDetail *string) *task.RunDetail {
//	if runDetail == nil || *runDetail == "" {
//		return nil
//	}
//
//	var runDetailDTO task.RunDetail
//	if err := sonic.Unmarshal([]byte(*runDetail), &runDetailDTO); err != nil {
//		logs.CtxError(ctx, "RunDetailPO2DTO sonic.Unmarshal err:%v", err)
//		return nil
//	}
//
//	return &runDetailDTO
//}
//
//// RunDetailPO2DTO 将JSON字符串转换为RunDetail结构体
//func BackfillRunDetailPO2DTO(ctx context.Context, runDetail *string) *task.BackfillDetail {
//	if runDetail == nil || *runDetail == "" {
//		return nil
//	}
//
//	var runDetailDTO task.BackfillDetail
//	if err := sonic.Unmarshal([]byte(*runDetail), &runDetailDTO); err != nil {
//		logs.CtxError(ctx, "RunDetailPO2DTO sonic.Unmarshal err:%v", err)
//		return nil
//	}
//
//	return &runDetailDTO
//}
//
//// TaskRunConfigPO2DTO 将JSON字符串转换为TaskRunConfig结构体
//func TaskRunConfigPO2DTO(ctx context.Context, runConfig *string) *task.TaskRunConfig {
//	if runConfig == nil || *runConfig == "" {
//		return nil
//	}
//
//	var runConfigDTO task.TaskRunConfig
//	if err := sonic.Unmarshal([]byte(*runConfig), &runConfigDTO); err != nil {
//		logs.CtxError(ctx, "TaskRunConfigPO2DTO sonic.Unmarshal err:%v", err)
//		return nil
//	}
//
//	return &runConfigDTO
//}
