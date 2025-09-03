// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"

	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ExptInsightAnalysisRecordDO2DTO(do *entity.ExptInsightAnalysisRecord) *domain_expt.ExptInsightAnalysisRecord {
	dto := &domain_expt.ExptInsightAnalysisRecord{
		RecordID:              do.ID,
		WorkspaceID:           do.SpaceID,
		ExptID:                do.ExptID,
		AnalysisStatus:        InsightAnalysisStatus2DTO(do.Status),
		AnalysisReportID:      do.AnalysisReportID,
		AnalysisReportContent: ptr.Of(do.AnalysisReportContent),
		BaseInfo: &domain_common.BaseInfo{
			CreatedBy: &domain_common.UserInfo{
				UserID: ptr.Of(do.CreatedBy),
			},
			CreatedAt: ptr.Of(do.CreatedAt.Unix()),
			UpdatedAt: ptr.Of(do.UpdatedAt.Unix()),
		},
	}
	return dto
}

func InsightAnalysisStatus2DTO(status entity.InsightAnalysisStatus) domain_expt.InsightAnalysisStatus {
	switch status {
	case entity.InsightAnalysisStatus_Unknown:
		return domain_expt.InsightAnalysisStatusUnknown
	case entity.InsightAnalysisStatus_Running:
		return domain_expt.InsightAnalysisStatusRunning
	case entity.InsightAnalysisStatus_Success:
		return domain_expt.InsightAnalysisStatusSuccess
	case entity.InsightAnalysisStatus_Failed:
		return domain_expt.InsightAnalysisStatusFailed
	default:
		return domain_expt.InsightAnalysisStatusUnknown
	}
}

func FeedbackActionType2DO(action domain_expt.FeedbackActionType) (entity.FeedbackActionType, error) {
	switch action {
	case domain_expt.FeedbackActionTypeUpvote:
		return entity.FeedbackActionType_Upvote, nil
	case domain_expt.FeedbackActionTypeCancelUpvote:
		return entity.FeedbackActionType_CancelUpvote, nil
	case domain_expt.FeedbackActionTypeDownvote:
		return entity.FeedbackActionType_Downvote, nil
	case domain_expt.FeedbackActionTypeCancelDownvote:
		return entity.FeedbackActionType_CancelDownvote, nil
	case domain_expt.FeedbackActionTypeCreateComment:
		return entity.FeedbackActionType_CreateComment, nil
	case domain_expt.FeedbackActionTypeDeleteComment:
		return entity.FeedbackActionType_Delete_Comment, nil

	default:
		return 0, fmt.Errorf("unknown feedback action type")
	}
}

func ExptInsightAnalysisFeedbackCommentDO2DTO(do *entity.ExptInsightAnalysisFeedbackComment) *domain_expt.ExptInsightAnalysisFeedbackComment {
	dto := &domain_expt.ExptInsightAnalysisFeedbackComment{
		CommentID:   do.ID,
		ExptID:      do.ExptID,
		WorkspaceID: do.SpaceID,
		RecordID:    do.AnalysisRecordID,
		Content:     do.Comment,
		BaseInfo: &domain_common.BaseInfo{
			CreatedBy: &domain_common.UserInfo{
				UserID: ptr.Of(do.CreatedBy),
			},
			CreatedAt: ptr.Of(do.CreatedAt.Unix()),
			UpdatedAt: ptr.Of(do.UpdatedAt.Unix()),
		},
	}
	return dto
}
