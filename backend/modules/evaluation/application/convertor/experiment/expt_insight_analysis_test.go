// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptInsightAnalysisRecordDO2DTO(t *testing.T) {
	tests := []struct {
		name     string
		do       *entity.ExptInsightAnalysisRecord
		expected *domain_expt.ExptInsightAnalysisRecord
	}{
		{
			name: "complete record with all fields",
			do: &entity.ExptInsightAnalysisRecord{
				ID:                    123,
				SpaceID:               456,
				ExptID:                789,
				Status:                entity.InsightAnalysisStatus_Success,
				ExptResultFilePath:    ptr.Of("/path/to/file"),
				AnalysisReportID:      ptr.Of(int64(999)),
				AnalysisReportContent: "Analysis report content",
				CreatedBy:             "user123",
				CreatedAt:             time.Unix(1640995200, 0), // 2022-01-01 00:00:00 UTC
				UpdatedAt:             time.Unix(1640995260, 0), // 2022-01-01 00:01:00 UTC
				ExptInsightAnalysisFeedback: entity.ExptInsightAnalysisFeedback{
					UpvoteCount:         10,
					DownvoteCount:       2,
					CurrentUserVoteType: entity.Upvote,
				},
			},
			expected: &domain_expt.ExptInsightAnalysisRecord{
				RecordID:              123,
				WorkspaceID:           456,
				ExptID:                789,
				AnalysisStatus:        domain_expt.InsightAnalysisStatusSuccess,
				AnalysisReportID:      ptr.Of(int64(999)),
				AnalysisReportContent: ptr.Of("Analysis report content"),
				ExptInsightAnalysisFeedback: &domain_expt.ExptInsightAnalysisFeedback{
					UpvoteCnt:           ptr.Of(int32(10)),
					DownvoteCnt:         ptr.Of(int32(2)),
					CurrentUserVoteType: ptr.Of(domain_expt.InsightAnalysisReportVoteTypeUpvote),
				},
				BaseInfo: &domain_common.BaseInfo{
					CreatedBy: &domain_common.UserInfo{
						UserID: ptr.Of("user123"),
					},
					CreatedAt: ptr.Of(int64(1640995200)),
					UpdatedAt: ptr.Of(int64(1640995260)),
				},
			},
		},
		{
			name: "minimal record with required fields only",
			do: &entity.ExptInsightAnalysisRecord{
				ID:                    1,
				SpaceID:               2,
				ExptID:                3,
				Status:                entity.InsightAnalysisStatus_Running,
				ExptResultFilePath:    nil,
				AnalysisReportID:      nil,
				AnalysisReportContent: "",
				CreatedBy:             "user456",
				CreatedAt:             time.Unix(1640995300, 0),
				UpdatedAt:             time.Unix(1640995300, 0),
				ExptInsightAnalysisFeedback: entity.ExptInsightAnalysisFeedback{
					UpvoteCount:         0,
					DownvoteCount:       0,
					CurrentUserVoteType: entity.None,
				},
			},
			expected: &domain_expt.ExptInsightAnalysisRecord{
				RecordID:              1,
				WorkspaceID:           2,
				ExptID:                3,
				AnalysisStatus:        domain_expt.InsightAnalysisStatusRunning,
				AnalysisReportID:      nil,
				AnalysisReportContent: ptr.Of(""),
				ExptInsightAnalysisFeedback: &domain_expt.ExptInsightAnalysisFeedback{
					UpvoteCnt:           ptr.Of(int32(0)),
					DownvoteCnt:         ptr.Of(int32(0)),
					CurrentUserVoteType: ptr.Of(domain_expt.InsightAnalysisReportVoteTypeNone),
				},
				BaseInfo: &domain_common.BaseInfo{
					CreatedBy: &domain_common.UserInfo{
						UserID: ptr.Of("user456"),
					},
					CreatedAt: ptr.Of(int64(1640995300)),
					UpdatedAt: ptr.Of(int64(1640995300)),
				},
			},
		},
		{
			name: "record with failed status",
			do: &entity.ExptInsightAnalysisRecord{
				ID:                    100,
				SpaceID:               200,
				ExptID:                300,
				Status:                entity.InsightAnalysisStatus_Failed,
				ExptResultFilePath:    ptr.Of("/failed/path"),
				AnalysisReportID:      nil,
				AnalysisReportContent: "Error occurred",
				CreatedBy:             "user789",
				CreatedAt:             time.Unix(1640995400, 0),
				UpdatedAt:             time.Unix(1640995500, 0),
				ExptInsightAnalysisFeedback: entity.ExptInsightAnalysisFeedback{
					UpvoteCount:         5,
					DownvoteCount:       15,
					CurrentUserVoteType: entity.Downvote,
				},
			},
			expected: &domain_expt.ExptInsightAnalysisRecord{
				RecordID:              100,
				WorkspaceID:           200,
				ExptID:                300,
				AnalysisStatus:        domain_expt.InsightAnalysisStatusFailed,
				AnalysisReportID:      nil,
				AnalysisReportContent: ptr.Of("Error occurred"),
				ExptInsightAnalysisFeedback: &domain_expt.ExptInsightAnalysisFeedback{
					UpvoteCnt:           ptr.Of(int32(5)),
					DownvoteCnt:         ptr.Of(int32(15)),
					CurrentUserVoteType: ptr.Of(domain_expt.InsightAnalysisReportVoteTypeDownvote),
				},
				BaseInfo: &domain_common.BaseInfo{
					CreatedBy: &domain_common.UserInfo{
						UserID: ptr.Of("user789"),
					},
					CreatedAt: ptr.Of(int64(1640995400)),
					UpdatedAt: ptr.Of(int64(1640995500)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExptInsightAnalysisRecordDO2DTO(tt.do)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptInsightAnalysisFeedbackDO2DTO(t *testing.T) {
	tests := []struct {
		name     string
		do       entity.ExptInsightAnalysisFeedback
		expected *domain_expt.ExptInsightAnalysisFeedback
	}{
		{
			name: "feedback with upvote",
			do: entity.ExptInsightAnalysisFeedback{
				UpvoteCount:         25,
				DownvoteCount:       3,
				CurrentUserVoteType: entity.Upvote,
			},
			expected: &domain_expt.ExptInsightAnalysisFeedback{
				UpvoteCnt:           ptr.Of(int32(25)),
				DownvoteCnt:         ptr.Of(int32(3)),
				CurrentUserVoteType: ptr.Of(domain_expt.InsightAnalysisReportVoteTypeUpvote),
			},
		},
		{
			name: "feedback with downvote",
			do: entity.ExptInsightAnalysisFeedback{
				UpvoteCount:         10,
				DownvoteCount:       20,
				CurrentUserVoteType: entity.Downvote,
			},
			expected: &domain_expt.ExptInsightAnalysisFeedback{
				UpvoteCnt:           ptr.Of(int32(10)),
				DownvoteCnt:         ptr.Of(int32(20)),
				CurrentUserVoteType: ptr.Of(domain_expt.InsightAnalysisReportVoteTypeDownvote),
			},
		},
		{
			name: "feedback with no vote",
			do: entity.ExptInsightAnalysisFeedback{
				UpvoteCount:         0,
				DownvoteCount:       0,
				CurrentUserVoteType: entity.None,
			},
			expected: &domain_expt.ExptInsightAnalysisFeedback{
				UpvoteCnt:           ptr.Of(int32(0)),
				DownvoteCnt:         ptr.Of(int32(0)),
				CurrentUserVoteType: ptr.Of(domain_expt.InsightAnalysisReportVoteTypeNone),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExptInsightAnalysisFeedbackDO2DTO(tt.do)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInsightAnalysisStatus2DTO(t *testing.T) {
	tests := []struct {
		name     string
		status   entity.InsightAnalysisStatus
		expected domain_expt.InsightAnalysisStatus
	}{
		{
			name:     "unknown status",
			status:   entity.InsightAnalysisStatus_Unknown,
			expected: domain_expt.InsightAnalysisStatusUnknown,
		},
		{
			name:     "running status",
			status:   entity.InsightAnalysisStatus_Running,
			expected: domain_expt.InsightAnalysisStatusRunning,
		},
		{
			name:     "success status",
			status:   entity.InsightAnalysisStatus_Success,
			expected: domain_expt.InsightAnalysisStatusSuccess,
		},
		{
			name:     "failed status",
			status:   entity.InsightAnalysisStatus_Failed,
			expected: domain_expt.InsightAnalysisStatusFailed,
		},
		{
			name:     "invalid status defaults to unknown",
			status:   entity.InsightAnalysisStatus(999), // invalid status
			expected: domain_expt.InsightAnalysisStatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InsightAnalysisStatus2DTO(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFeedbackActionType2DO(t *testing.T) {
	tests := []struct {
		name        string
		action      domain_expt.FeedbackActionType
		expected    entity.FeedbackActionType
		expectedErr bool
	}{
		{
			name:        "upvote action",
			action:      domain_expt.FeedbackActionTypeUpvote,
			expected:    entity.FeedbackActionType_Upvote,
			expectedErr: false,
		},
		{
			name:        "cancel upvote action",
			action:      domain_expt.FeedbackActionTypeCancelUpvote,
			expected:    entity.FeedbackActionType_CancelUpvote,
			expectedErr: false,
		},
		{
			name:        "downvote action",
			action:      domain_expt.FeedbackActionTypeDownvote,
			expected:    entity.FeedbackActionType_Downvote,
			expectedErr: false,
		},
		{
			name:        "cancel downvote action",
			action:      domain_expt.FeedbackActionTypeCancelDownvote,
			expected:    entity.FeedbackActionType_CancelDownvote,
			expectedErr: false,
		},
		{
			name:        "create comment action",
			action:      domain_expt.FeedbackActionTypeCreateComment,
			expected:    entity.FeedbackActionType_CreateComment,
			expectedErr: false,
		},
		{
			name:        "update comment action",
			action:      domain_expt.FeedbackActionTypeUpdateComment,
			expected:    entity.FeedbackActionType_Update_Comment,
			expectedErr: false,
		},
		{
			name:        "delete comment action",
			action:      domain_expt.FeedbackActionTypeDeleteComment,
			expected:    entity.FeedbackActionType_Delete_Comment,
			expectedErr: false,
		},
		{
			name:        "unknown action type",
			action:      "unknown_action",
			expected:    0,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FeedbackActionType2DO(tt.action)
			if tt.expectedErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unknown feedback action type")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestInsightAnalysisReportVoteType2DTO(t *testing.T) {
	tests := []struct {
		name     string
		voteType entity.InsightAnalysisReportVoteType
		expected domain_expt.InsightAnalysisReportVoteType
	}{
		{
			name:     "none vote type",
			voteType: entity.None,
			expected: domain_expt.InsightAnalysisReportVoteTypeNone,
		},
		{
			name:     "upvote type",
			voteType: entity.Upvote,
			expected: domain_expt.InsightAnalysisReportVoteTypeUpvote,
		},
		{
			name:     "downvote type",
			voteType: entity.Downvote,
			expected: domain_expt.InsightAnalysisReportVoteTypeDownvote,
		},
		{
			name:     "invalid vote type defaults to none",
			voteType: entity.InsightAnalysisReportVoteType(999), // invalid type
			expected: domain_expt.InsightAnalysisReportVoteTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InsightAnalysisReportVoteType2DTO(tt.voteType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExptInsightAnalysisFeedbackCommentDO2DTO(t *testing.T) {
	tests := []struct {
		name     string
		do       *entity.ExptInsightAnalysisFeedbackComment
		expected *domain_expt.ExptInsightAnalysisFeedbackComment
	}{
		{
			name: "complete comment",
			do: &entity.ExptInsightAnalysisFeedbackComment{
				ID:               123,
				SpaceID:          456,
				ExptID:           789,
				AnalysisRecordID: 999,
				Comment:          "This is a test comment",
				CreatedBy:        "user123",
				CreatedAt:        time.Unix(1640995200, 0),
				UpdatedAt:        time.Unix(1640995260, 0),
			},
			expected: &domain_expt.ExptInsightAnalysisFeedbackComment{
				CommentID:   123,
				ExptID:      789,
				WorkspaceID: 456,
				RecordID:    999,
				Content:     "This is a test comment",
				BaseInfo: &domain_common.BaseInfo{
					CreatedBy: &domain_common.UserInfo{
						UserID: ptr.Of("user123"),
					},
					CreatedAt: ptr.Of(int64(1640995200)),
					UpdatedAt: ptr.Of(int64(1640995260)),
				},
			},
		},
		{
			name: "empty comment",
			do: &entity.ExptInsightAnalysisFeedbackComment{
				ID:               1,
				SpaceID:          2,
				ExptID:           3,
				AnalysisRecordID: 4,
				Comment:          "",
				CreatedBy:        "user456",
				CreatedAt:        time.Unix(1640995300, 0),
				UpdatedAt:        time.Unix(1640995300, 0),
			},
			expected: &domain_expt.ExptInsightAnalysisFeedbackComment{
				CommentID:   1,
				ExptID:      3,
				WorkspaceID: 2,
				RecordID:    4,
				Content:     "",
				BaseInfo: &domain_common.BaseInfo{
					CreatedBy: &domain_common.UserInfo{
						UserID: ptr.Of("user456"),
					},
					CreatedAt: ptr.Of(int64(1640995300)),
					UpdatedAt: ptr.Of(int64(1640995300)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExptInsightAnalysisFeedbackCommentDO2DTO(tt.do)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test edge cases and error conditions
func TestExptInsightAnalysisRecordDO2DTO_EdgeCases(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		// This should not panic, but the function doesn't handle nil input
		// In a real scenario, we might want to add nil checks
		assert.Panics(t, func() {
			ExptInsightAnalysisRecordDO2DTO(nil)
		})
	})

	t.Run("zero values", func(t *testing.T) {
		do := &entity.ExptInsightAnalysisRecord{}
		result := ExptInsightAnalysisRecordDO2DTO(do)
		assert.NotNil(t, result)
		assert.Equal(t, int64(0), result.RecordID)
		assert.Equal(t, int64(0), result.WorkspaceID)
		assert.Equal(t, int64(0), result.ExptID)
		assert.Equal(t, domain_expt.InsightAnalysisStatusUnknown, result.AnalysisStatus)
	})
}

func TestExptInsightAnalysisFeedbackCommentDO2DTO_EdgeCases(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Panics(t, func() {
			ExptInsightAnalysisFeedbackCommentDO2DTO(nil)
		})
	})

	t.Run("zero values", func(t *testing.T) {
		do := &entity.ExptInsightAnalysisFeedbackComment{}
		result := ExptInsightAnalysisFeedbackCommentDO2DTO(do)
		assert.NotNil(t, result)
		assert.Equal(t, int64(0), result.CommentID)
		assert.Equal(t, int64(0), result.ExptID)
		assert.Equal(t, int64(0), result.WorkspaceID)
		assert.Equal(t, int64(0), result.RecordID)
		assert.Equal(t, "", result.Content)
	})
}
