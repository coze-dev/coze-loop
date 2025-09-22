// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ExptInsightAnalysisRecordDOToPO(record *entity.ExptInsightAnalysisRecord) *model.ExptInsightAnalysisRecord {
	return &model.ExptInsightAnalysisRecord{
		ID:                 record.ID,
		SpaceID:            record.SpaceID,
		ExptID:             record.ExptID,
		Status:             int32(record.Status),
		ExptResultFilePath: record.ExptResultFilePath,
		AnalysisReportID:   record.AnalysisReportID,
		CreatedBy:          record.CreatedBy,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func ExptInsightAnalysisRecordPOToDO(record *model.ExptInsightAnalysisRecord) *entity.ExptInsightAnalysisRecord {
	return &entity.ExptInsightAnalysisRecord{
		ID:                 record.ID,
		SpaceID:            record.SpaceID,
		ExptID:             record.ExptID,
		Status:             entity.InsightAnalysisStatus(record.Status),
		ExptResultFilePath: record.ExptResultFilePath,
		AnalysisReportID:   record.AnalysisReportID,
		CreatedBy:          record.CreatedBy,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

func ExptInsightAnalysisFeedbackCommentDOToPO(comment *entity.ExptInsightAnalysisFeedbackComment) *model.ExptInsightAnalysisFeedbackComment {
	return &model.ExptInsightAnalysisFeedbackComment{
		ID:               comment.ID,
		SpaceID:          comment.SpaceID,
		ExptID:           comment.ExptID,
		AnalysisRecordID: ptr.Of(comment.AnalysisRecordID),
		Comment:          ptr.Of(comment.Comment),
		CreatedBy:        comment.CreatedBy,
		CreatedAt:        comment.CreatedAt,
		UpdatedAt:        comment.UpdatedAt,
	}
}

func ExptInsightAnalysisFeedbackCommentPOToDO(comment *model.ExptInsightAnalysisFeedbackComment) *entity.ExptInsightAnalysisFeedbackComment {
	return &entity.ExptInsightAnalysisFeedbackComment{
		ID:               comment.ID,
		SpaceID:          comment.SpaceID,
		ExptID:           comment.ExptID,
		AnalysisRecordID: ptr.From(comment.AnalysisRecordID),
		Comment:          ptr.From(comment.Comment),
		CreatedBy:        comment.CreatedBy,
		CreatedAt:        comment.CreatedAt,
		UpdatedAt:        comment.UpdatedAt,
	}
}

func ExptInsightAnalysisFeedbackVoteDOToPO(vote *entity.ExptInsightAnalysisFeedbackVote) *model.ExptInsightAnalysisFeedbackVote {
	return &model.ExptInsightAnalysisFeedbackVote{
		ID:               vote.ID,
		SpaceID:          vote.SpaceID,
		ExptID:           vote.ExptID,
		VoteType:         int32(vote.VoteType),
		AnalysisRecordID: ptr.Of(vote.AnalysisRecordID),
		CreatedBy:        vote.CreatedBy,
		CreatedAt:        vote.CreatedAt,
		UpdatedAt:        vote.UpdatedAt,
	}
}

func ExptInsightAnalysisFeedbackVotePOToDO(vote *model.ExptInsightAnalysisFeedbackVote) *entity.ExptInsightAnalysisFeedbackVote {
	if vote == nil {
		return nil
	}
	return &entity.ExptInsightAnalysisFeedbackVote{
		ID:               vote.ID,
		SpaceID:          vote.SpaceID,
		ExptID:           vote.ExptID,
		VoteType:         entity.InsightAnalysisReportVoteType(vote.VoteType),
		AnalysisRecordID: ptr.From(vote.AnalysisRecordID),
		CreatedBy:        vote.CreatedBy,
		CreatedAt:        vote.CreatedAt,
		UpdatedAt:        vote.UpdatedAt,
	}
}
