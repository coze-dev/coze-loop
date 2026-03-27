package convertor

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
)

func TestToolPO2DO_NilBasicPO(t *testing.T) {
	t.Parallel()
	result := ToolPO2DO(nil, nil)
	assert.Nil(t, result)
}

func TestToolPO2DO_BasicOnly(t *testing.T) {
	t.Parallel()
	now := time.Now()
	basicPO := &model.ToolBasic{
		ID:                     1,
		SpaceID:                100,
		Name:                   "example-tool",
		Description:            "desc",
		LatestCommittedVersion: "1.0.0",
		CreatedBy:              "user-1",
		UpdatedBy:              "user-2",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	result := ToolPO2DO(basicPO, nil)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, int64(100), result.SpaceID)
	assert.NotNil(t, result.ToolBasic)
	assert.Equal(t, "example-tool", result.ToolBasic.Name)
	assert.Equal(t, "desc", result.ToolBasic.Description)
	assert.Equal(t, "1.0.0", result.ToolBasic.LatestCommittedVersion)
	assert.Equal(t, "user-1", result.ToolBasic.CreatedBy)
	assert.Equal(t, "user-2", result.ToolBasic.UpdatedBy)
	assert.Equal(t, now, result.ToolBasic.CreatedAt)
	assert.Equal(t, now, result.ToolBasic.UpdatedAt)
	assert.Nil(t, result.ToolCommit)
}

func TestToolPO2DO_WithCommit(t *testing.T) {
	t.Parallel()
	now := time.Now()
	basicPO := &model.ToolBasic{
		ID:      1,
		SpaceID: 100,
		Name:    "example-tool",
	}
	commitPO := &model.ToolCommit{
		ToolID:      1,
		Version:     "1.0.0",
		BaseVersion: "0.0.1",
		CommittedBy: "user-1",
		Content:     lo.ToPtr("tool content"),
		Description: lo.ToPtr("commit desc"),
		CreatedAt:   now,
	}
	result := ToolPO2DO(basicPO, commitPO)
	assert.NotNil(t, result)
	assert.NotNil(t, result.ToolCommit)
	assert.Equal(t, "1.0.0", result.ToolCommit.CommitInfo.Version)
	assert.Equal(t, "0.0.1", result.ToolCommit.CommitInfo.BaseVersion)
	assert.Equal(t, "commit desc", result.ToolCommit.CommitInfo.Description)
	assert.Equal(t, "user-1", result.ToolCommit.CommitInfo.CommittedBy)
	assert.Equal(t, now, result.ToolCommit.CommitInfo.CommittedAt)
	assert.Equal(t, "tool content", result.ToolCommit.ToolDetail.Content)
}

func TestToolCommitPO2DO_Nil(t *testing.T) {
	t.Parallel()
	result := ToolCommitPO2DO(nil)
	assert.Nil(t, result)
}

func TestToolCommitPO2DO_NilContentAndDescription(t *testing.T) {
	t.Parallel()
	commitPO := &model.ToolCommit{
		ToolID:      1,
		Version:     "1.0.0",
		BaseVersion: "",
		CommittedBy: "user-1",
		Content:     nil,
		Description: nil,
		CreatedAt:   time.Now(),
	}
	result := ToolCommitPO2DO(commitPO)
	assert.NotNil(t, result)
	assert.Equal(t, "", result.CommitInfo.Description)
	assert.Equal(t, "", result.ToolDetail.Content)
}

func TestToolDO2BasicPO_Nil(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		do   *toolmgmt.Tool
	}{
		{name: "nil tool", do: nil},
		{name: "nil tool basic", do: &toolmgmt.Tool{ID: 1, ToolBasic: nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ToolDO2BasicPO(tt.do)
			assert.Nil(t, result)
		})
	}
}

func TestToolDO2BasicPO_FullMapping(t *testing.T) {
	t.Parallel()
	now := time.Now()
	do := &toolmgmt.Tool{
		ID:      1,
		SpaceID: 100,
		ToolBasic: &toolmgmt.ToolBasic{
			Name:                   "example-tool",
			Description:            "desc",
			LatestCommittedVersion: "2.0.0",
			CreatedBy:              "user-1",
			UpdatedBy:              "user-2",
			CreatedAt:              now,
			UpdatedAt:              now,
		},
	}
	result := ToolDO2BasicPO(do)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, int64(100), result.SpaceID)
	assert.Equal(t, "example-tool", result.Name)
	assert.Equal(t, "desc", result.Description)
	assert.Equal(t, "2.0.0", result.LatestCommittedVersion)
	assert.Equal(t, "user-1", result.CreatedBy)
	assert.Equal(t, "user-2", result.UpdatedBy)
	assert.Equal(t, now, result.CreatedAt)
	assert.Equal(t, now, result.UpdatedAt)
}
