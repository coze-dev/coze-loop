// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type InsightAnalysisStatus int

const (
	InsightAnalysisStatus_Unknown InsightAnalysisStatus = 0
	InsightAnalysisStatus_Running InsightAnalysisStatus = 1
	InsightAnalysisStatus_Success InsightAnalysisStatus = 2
	InsightAnalysisStatus_Failed  InsightAnalysisStatus = 3
)

const (
	InsightAnalysisRunningTimeout = 2 * time.Hour
)

// InsightAnalysisScope 智能解读范围（Experiment=全实验报告 / Row=单行）
type InsightAnalysisScope string

const (
	InsightAnalysisScope_Experiment InsightAnalysisScope = "Experiment"
	InsightAnalysisScope_Row        InsightAnalysisScope = "Row"
)

type ExptInsightAnalysisRecord struct {
	ID                    int64
	SpaceID               int64
	ExptID                int64
	Status                InsightAnalysisStatus
	ExptResultFilePath    *string
	AnalysisReportID      *int64
	AnalysisReportContent string
	AnalysisReportIndex   []*InsightAnalysisReportIndex
	CreatedBy             string
	CreatedAt             time.Time
	UpdatedAt             time.Time

	// Scope 标识该 record 的范围（Experiment / Row）。
	// 老数据为空时按 Experiment 处理（前端兼容降级）。
	Scope  InsightAnalysisScope
	ItemID *int64 // Scope=Row 时填，标识评测集 item
	TurnID *int64 // Scope=Row 时填，标识 item 内 turn

	ExptInsightAnalysisFeedback ExptInsightAnalysisFeedback
}

type ExptInsightAnalysisFeedbackComment struct {
	ID               int64
	SpaceID          int64
	ExptID           int64
	AnalysisRecordID int64
	Comment          string
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// InsightAnalysisReportVoteType 洞察报告反馈类型
type InsightAnalysisReportVoteType int

const (
	None     InsightAnalysisReportVoteType = 0
	Upvote   InsightAnalysisReportVoteType = 1
	Downvote InsightAnalysisReportVoteType = 2
)

type ExptInsightAnalysisFeedbackVote struct {
	ID               int64
	SpaceID          int64
	ExptID           int64
	AnalysisRecordID int64
	VoteType         InsightAnalysisReportVoteType
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ExptInsightAnalysisFeedback struct {
	UpvoteCount         int64
	DownvoteCount       int64
	CurrentUserVoteType InsightAnalysisReportVoteType
}

type FeedbackActionType int

const (
	FeedbackActionType_Upvote         FeedbackActionType = 1
	FeedbackActionType_CancelUpvote   FeedbackActionType = 2
	FeedbackActionType_Downvote       FeedbackActionType = 3
	FeedbackActionType_CancelDownvote FeedbackActionType = 4
	FeedbackActionType_CreateComment  FeedbackActionType = 5
	FeedbackActionType_Update_Comment FeedbackActionType = 6
	FeedbackActionType_Delete_Comment FeedbackActionType = 7
)

type ExptInsightAnalysisFeedbackParam struct {
	SpaceID            int64
	ExptID             int64
	AnalysisRecordID   int64
	FeedbackActionType FeedbackActionType
	Comment            *string
	CommentID          *int64
	Session            *Session
}

type ReportStatus int64

const (
	// 未定义
	ReportStatus_Unknown ReportStatus = 0
	// 进行中
	ReportStatus_Running ReportStatus = 1
	// 生成成功
	ReportStatus_Success ReportStatus = 2
	// 生成失败
	ReportStatus_Failed ReportStatus = 3
)

type InsightAnalysisReportIndex struct {
	ID    string
	Title string
}
