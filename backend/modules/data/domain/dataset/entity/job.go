// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
)

type IOJob struct {
	ID      int64
	AppID   *int32
	SpaceID int64
	// 导入导出到文件时，为数据集 ID；数据集间转移时，为目标数据集 ID
	DatasetID int64
	JobType   JobType
	Source    *DatasetIOEndpoint
	Target    *DatasetIOEndpoint
	// 字段映射
	FieldMappings []*FieldMapping
	Option        *DatasetIOJobOption
	/* 运行数据, [20, 100) */
	Status   *JobStatus
	Progress *DatasetIOJobProgress
	Errors   []*ItemErrorGroup
	/* 通用信息 */
	CreatedBy *string
	CreatedAt *int64
	UpdatedBy *string
	UpdatedAt *int64
	StartedAt *int64
	EndedAt   *int64
}

type DatasetIOJobOption struct {
	// 覆盖数据集
	OverwriteDataset *bool
}

type FieldMapping struct {
	Source string
	Target string
}

type DatasetIOEndpoint struct {
	File    *DatasetIOFile
	Dataset *DatasetIODataset
}

type DatasetIODataset struct {
	SpaceID   *int64
	DatasetID int64
	VersionID *int64
}

type DatasetIOFile struct {
	Provider entity.Provider
	Path     string
	// 数据文件的格式
	Format *FileFormat
	// 压缩包格式
	CompressFormat *FileFormat
	// path 为文件夹或压缩包时，数据文件列表, 服务端设置
	Files []string
}

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

func (p JobStatus) String() string {
	switch p {
	case JobStatus_Undefined:
		return "Undefined"
	case JobStatus_Pending:
		return "Pending"
	case JobStatus_Running:
		return "Running"
	case JobStatus_Completed:
		return "Completed"
	case JobStatus_Failed:
		return "Failed"
	case JobStatus_Cancelled:
		return "Cancelled"
	}
	return "<UNSET>"
}

type FileFormat int64

const (
	FileFormat_JSONL   FileFormat = 1
	FileFormat_Parquet FileFormat = 2
	FileFormat_CSV     FileFormat = 3
	/*[100, 200) 压缩格式*/
	FileFormat_ZIP FileFormat = 100
)

func (p FileFormat) String() string {
	switch p {
	case FileFormat_JSONL:
		return "JSONL"
	case FileFormat_Parquet:
		return "Parquet"
	case FileFormat_CSV:
		return "CSV"
	case FileFormat_ZIP:
		return "ZIP"
	}
	return "<UNSET>"
}

func IsJobTerminal(status JobStatus) bool {
	switch status {
	case JobStatus_Completed, JobStatus_Failed, JobStatus_Cancelled:
		return true
	default:
		return false
	}
}

type JobRunType string

const (
	DatasetIOJob       JobRunType = "dataset_io_job"       // 新版
	DatasetSnapshotJob JobRunType = "dataset_snapshot_job" // 新版
)

type JobRunMessage struct {
	Type     JobRunType        `json:"type"`
	SpaceID  int64             `json:"space_id"`
	TaskID   int64             `json:"task_id"` // Deprecated, use JobID instead
	RunID    int64             `json:"run_id"`  // 多次运行的任务, 可指定 RunID
	JobID    int64             `json:"job_id"`  // 任务所属的 JobID
	Extra    map[string]string `json:"extra"`   // 任务运行的额外信息
	Operator string            `json:"operator"`
}

type DatasetIOJobProgress struct {
	// 总量
	Total *int64
	// 已处理数量
	Processed *int64
	// 已成功处理的数量
	Added *int64
	/*子任务*/
	Name *string
	// 子任务的进度
	SubProgresses []*DatasetIOJobProgress
}

type JobType int64

const (
	JobType_ImportFromFile  JobType = 1
	JobType_ExportToFile    JobType = 2
	JobType_ExportToDataset JobType = 3
)

func (p JobType) String() string {
	switch p {
	case JobType_ImportFromFile:
		return "ImportFromFile"
	case JobType_ExportToFile:
		return "ExportToFile"
	case JobType_ExportToDataset:
		return "ExportToDataset"
	}
	return "<UNSET>"
}

func (p *IOJob) IsSetStartedAt() bool {
	return p.StartedAt != nil
}

func (p *IOJob) IsSetEndedAt() bool {
	return p.EndedAt != nil
}

func JobTypeFromString(s string) (JobType, error) {
	switch s {
	case "ImportFromFile":
		return JobType_ImportFromFile, nil
	case "ExportToFile":
		return JobType_ExportToFile, nil
	case "ExportToDataset":
		return JobType_ExportToDataset, nil
	}
	return JobType(0), fmt.Errorf("not a valid JobType string")
}

func JobStatusFromString(s string) (JobStatus, error) {
	switch s {
	case "Undefined":
		return JobStatus_Undefined, nil
	case "Pending":
		return JobStatus_Pending, nil
	case "Running":
		return JobStatus_Running, nil
	case "Completed":
		return JobStatus_Completed, nil
	case "Failed":
		return JobStatus_Failed, nil
	case "Cancelled":
		return JobStatus_Cancelled, nil
	}
	return JobStatus(0), fmt.Errorf("not a valid JobStatus string")
}

func (p *IOJob) GetID() (v int64) {
	if p != nil {
		return p.ID
	}
	return
}

func (p *IOJob) SetID(val int64) {
	p.ID = val
}
