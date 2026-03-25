// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// JobType 通用任务类型
type JobType int64

const (
	JobType_ImportFromFile  JobType = 1
	JobType_ExportToFile    JobType = 2
	JobType_ExportToDataset JobType = 3
)

// JobStatus 通用任务状态
type JobStatus int64

const (
	JobStatus_Undefined JobStatus = 0
	// 待处理
	JobStatus_Pending JobStatus = 1
	// 处理中
	JobStatus_Running JobStatus = 2
	// 已完成
	JobStatus_Completed JobStatus = 3
	// 失败
	JobStatus_Failed JobStatus = 4
	// 已取消
	JobStatus_Cancelled JobStatus = 5
)

type DatasetIOJobOption struct {
	// 覆盖数据集
	OverwriteDataset *bool `json:"overwrite_dataset,omitempty"`
}

type DatasetIOJobProgress struct {
	// 总量
	Total *int64 `json:"total,omitempty"`
	// 已处理数量
	Processed *int64 `json:"processed,omitempty"`
	// 已成功处理的数量
	Added *int64 `json:"added,omitempty"`
	/*子任务*/
	Name *string `json:"name,omitempty"`
	// 子任务的进度
	SubProgresses []*DatasetIOJobProgress `json:"sub_progresses,omitempty"`
}

// DatasetIOJob 数据集导入导出任务
type DatasetIOJob struct {
	ID      int64  `json:"id"`
	AppID   *int32 `json:"app_id,omitempty"`
	SpaceID int64  `json:"space_id"`
	// 导入导出到文件时，为数据集 ID；数据集间转移时，为目标数据集 ID
	DatasetID int64              `json:"dataset_id"`
	JobType   JobType            `json:"job_type"`
	Source    *DatasetIOEndpoint `json:"source"`
	Target    *DatasetIOEndpoint `json:"target"`
	// 字段映射
	FieldMappings []*FieldMapping     `json:"field_mappings,omitempty"`
	Option        *DatasetIOJobOption `json:"option,omitempty"`
	/* 运行数据, [20, 100) */
	Status   *JobStatus            `json:"status,omitempty"`
	Progress *DatasetIOJobProgress `json:"progress,omitempty"`
	Errors   []*ItemErrorGroup     `json:"errors,omitempty"`
	/* 通用信息 */
	CreatedBy *string `json:"created_by,omitempty"`
	CreatedAt *int64  `json:"created_at,omitempty"`
	UpdatedBy *string `json:"updated_by,omitempty"`
	UpdatedAt *int64  `json:"updated_at,omitempty"`
	StartedAt *int64  `json:"started_at,omitempty"`
	EndedAt   *int64  `json:"ended_at,omitempty"`
}

type ImportEvaluationSetParam struct {
	WorkspaceID     int64
	EvaluationSetID int64
	File            *DatasetIOFile
	FieldMappings   []*FieldMapping
	Option          *DatasetIOJobOption
}
