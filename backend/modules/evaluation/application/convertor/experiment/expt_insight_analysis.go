// Copyright (c) 2025 coze-dev Authors
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
		RecordID:                    do.ID,
		WorkspaceID:                 do.SpaceID,
		ExptID:                      do.ExptID,
		AnalysisStatus:              InsightAnalysisStatus2DTO(do.Status),
		AnalysisReportID:            do.AnalysisReportID,
		AnalysisReportContent:       ptr.Of(do.AnalysisReportContent),
		AnalysisReportIndex:         AnalysisReportIndex2DTO(do.AnalysisReportIndex),
		ExptInsightAnalysisFeedback: ExptInsightAnalysisFeedbackDO2DTO(do.ExptInsightAnalysisFeedback),
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

func AnalysisReportIndex2DTO(index []*entity.InsightAnalysisReportIndex) []*domain_expt.ExptInsightAnalysisIndex {
	if len(index) == 0 {
		return nil
	}
	dto := make([]*domain_expt.ExptInsightAnalysisIndex, 0, len(index))
	for _, item := range index {
		dto = append(dto, &domain_expt.ExptInsightAnalysisIndex{
			ID:    ptr.Of(item.ID),
			Title: ptr.Of(item.Title),
		})
	}
	return dto
}

func ExptInsightAnalysisFeedbackDO2DTO(do entity.ExptInsightAnalysisFeedback) *domain_expt.ExptInsightAnalysisFeedback {
	dto := &domain_expt.ExptInsightAnalysisFeedback{
		UpvoteCnt:           ptr.Of(int32(do.UpvoteCount)),
		DownvoteCnt:         ptr.Of(int32(do.DownvoteCount)),
		CurrentUserVoteType: ptr.Of(InsightAnalysisReportVoteType2DTO(do.CurrentUserVoteType)),
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
	case domain_expt.FeedbackActionTypeUpdateComment:
		return entity.FeedbackActionType_Update_Comment, nil
	case domain_expt.FeedbackActionTypeDeleteComment:
		return entity.FeedbackActionType_Delete_Comment, nil

	default:
		return 0, fmt.Errorf("unknown feedback action type")
	}
}

func InsightAnalysisReportVoteType2DTO(voteType entity.InsightAnalysisReportVoteType) domain_expt.InsightAnalysisReportVoteType {
	switch voteType {
	case entity.None:
		return domain_expt.InsightAnalysisReportVoteTypeNone
	case entity.Upvote:
		return domain_expt.InsightAnalysisReportVoteTypeUpvote
	case entity.Downvote:
		return domain_expt.InsightAnalysisReportVoteTypeDownvote
	default:
		return domain_expt.InsightAnalysisReportVoteTypeNone
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
