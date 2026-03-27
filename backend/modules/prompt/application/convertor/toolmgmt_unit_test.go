package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
)

func TestToolMgmtDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := ToolMgmtDO2DTO(nil)
	assert.Nil(t, result)
}

func TestToolMgmtDO2DTO_NilSubfields(t *testing.T) {
	t.Parallel()
	do := &toolmgmt.Tool{
		ID:         10,
		SpaceID:    20,
		ToolBasic:  nil,
		ToolCommit: nil,
	}
	result := ToolMgmtDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, int64(10), result.GetID())
	assert.Equal(t, int64(20), result.GetWorkspaceID())
	assert.Nil(t, result.ToolBasic)
	assert.Nil(t, result.ToolCommit)
}

func TestToolMgmtDO2DTO_FullFields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	do := &toolmgmt.Tool{
		ID:      1,
		SpaceID: 100,
		ToolBasic: &toolmgmt.ToolBasic{
			Name:                   "example-tool",
			Description:            "desc",
			LatestCommittedVersion: "1.0.0",
			CreatedBy:              "user-1",
			UpdatedBy:              "user-2",
			CreatedAt:              now,
			UpdatedAt:              now,
		},
		ToolCommit: &toolmgmt.ToolCommit{
			CommitInfo: &toolmgmt.CommitInfo{
				Version:     "1.0.0",
				BaseVersion: "0.0.1",
				Description: "initial",
				CommittedBy: "user-1",
				CommittedAt: now,
			},
			ToolDetail: &toolmgmt.ToolDetail{
				Content: "tool content",
			},
		},
	}
	result := ToolMgmtDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, "example-tool", result.ToolBasic.GetName())
	assert.Equal(t, "desc", result.ToolBasic.GetDescription())
	assert.Equal(t, "1.0.0", result.ToolBasic.GetLatestCommittedVersion())
	assert.Equal(t, "user-1", result.ToolBasic.GetCreatedBy())
	assert.Equal(t, "user-2", result.ToolBasic.GetUpdatedBy())
	assert.Equal(t, now.UnixMilli(), result.ToolBasic.GetCreatedAt())
	assert.Equal(t, now.UnixMilli(), result.ToolBasic.GetUpdatedAt())

	assert.NotNil(t, result.ToolCommit)
	assert.Equal(t, "1.0.0", result.ToolCommit.CommitInfo.GetVersion())
	assert.Equal(t, "0.0.1", result.ToolCommit.CommitInfo.GetBaseVersion())
	assert.Equal(t, "initial", result.ToolCommit.CommitInfo.GetDescription())
	assert.Equal(t, "tool content", result.ToolCommit.Detail.GetContent())
}

func TestToolMgmtBasicDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := ToolMgmtBasicDO2DTO(nil)
	assert.Nil(t, result)
}

func TestToolMgmtCommitDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := ToolMgmtCommitDO2DTO(nil)
	assert.Nil(t, result)
}

func TestToolMgmtCommitDO2DTO_NilSubfields(t *testing.T) {
	t.Parallel()
	do := &toolmgmt.ToolCommit{
		CommitInfo: nil,
		ToolDetail: nil,
	}
	result := ToolMgmtCommitDO2DTO(do)
	assert.NotNil(t, result)
	assert.Nil(t, result.CommitInfo)
	assert.Nil(t, result.Detail)
}

func TestToolMgmtCommitInfoDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := ToolMgmtCommitInfoDO2DTO(nil)
	assert.Nil(t, result)
}

func TestToolMgmtDetailDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	result := ToolMgmtDetailDO2DTO(nil)
	assert.Nil(t, result)
}

func TestToolMgmtDetailDO2DTO_EmptyContent(t *testing.T) {
	t.Parallel()
	do := &toolmgmt.ToolDetail{Content: ""}
	result := ToolMgmtDetailDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, "", result.GetContent())
}

// --- BatchToolMgmtDO2DTO tests ---

func TestBatchToolMgmtDO2DTO_EmptySlice(t *testing.T) {
	t.Parallel()
	result := BatchToolMgmtDO2DTO(nil)
	assert.Nil(t, result)

	result = BatchToolMgmtDO2DTO([]*toolmgmt.Tool{})
	assert.Nil(t, result)
}

func TestBatchToolMgmtDO2DTO_AllNil(t *testing.T) {
	t.Parallel()
	result := BatchToolMgmtDO2DTO([]*toolmgmt.Tool{nil, nil, nil})
	assert.Nil(t, result)
}

func TestBatchToolMgmtDO2DTO_MixedNilAndValid(t *testing.T) {
	t.Parallel()
	dos := []*toolmgmt.Tool{
		nil,
		{ID: 1, SpaceID: 100},
		nil,
		{ID: 2, SpaceID: 200},
	}
	result := BatchToolMgmtDO2DTO(dos)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(1), result[0].GetID())
	assert.Equal(t, int64(2), result[1].GetID())
}
