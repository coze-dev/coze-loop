// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

// ItemCompleteEvent 单行评测完成事件，供离线分析侧消费后回查完整数据。
// ID 均以 string 传输，避免下游 JSON 解析丢精度。
type ItemCompleteEvent struct {
	EvalTargetWorkspaceID string `json:"eval_target_workspace_id"` // 评测对象所在空间
	EvalTargetID          string `json:"eval_target_id"`           // 评测对象 ID（应用注册的 ID）
	ExptWorkspaceID       string `json:"expt_workspace_id"`        // 实验发起的空间
	ExptID                string `json:"expt_id"`                  // 实验 ID
	ExptRunID             string `json:"expt_run_id"`              // 实验单次执行的 ID
	DatasetWorkspaceID    string `json:"dataset_workspace_id"`     // 数据集所在空间
	DatasetID             string `json:"dataset_id"`               // 数据集 ID
	DatasetVersionID      string `json:"dataset_version_id"`       // 数据集版本 ID
	DatasetKey            string `json:"dataset_key"`              // 数据集唯一 Key（预期不可修改）
	DatasetVersionName    string `json:"dataset_version_name"`     // 数据集版本名字，如 0.0.1
	ExperimentGroupKey    string `json:"experiment_group_key"`     // 实验组 Key（关联同组实验），默认为实验 ID，空间内唯一
	ItemID                string `json:"item_id"`                  // 数据集某一行的 ID
	ItemKey               string `json:"item_key"`                 // 评测集 item 的实体 ItemKey（下游 data 服务写入）；直接透传，为空则空、不降级
}

type IItemCompletePublisher interface {
	PublishItemComplete(ctx context.Context, event *ItemCompleteEvent) error
}
