// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	model2 "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/stretchr/testify/assert"
)

type columnExtractDaoStub struct {
	getResp     *model2.ObservabilityColumnExtractConfig
	getErr      error
	createErr   error
	updateErr   error
	lastCreated *model2.ObservabilityColumnExtractConfig
	lastUpdated *model2.ObservabilityColumnExtractConfig
}

func (c *columnExtractDaoStub) GetColumnExtractConfig(ctx context.Context, workspaceID int64, platformType, spanListType, agentName string) (*model2.ObservabilityColumnExtractConfig, error) {
	return c.getResp, c.getErr
}

func (c *columnExtractDaoStub) UpdateColumnExtractConfig(ctx context.Context, po *model2.ObservabilityColumnExtractConfig) error {
	c.lastUpdated = po
	return c.updateErr
}

func (c *columnExtractDaoStub) CreateColumnExtractConfig(ctx context.Context, po *model2.ObservabilityColumnExtractConfig) error {
	c.lastCreated = po
	return c.createErr
}

func TestColumnExtractConfigRepoImpl_UpsertColumnExtractConfig(t *testing.T) {
	{
		stub := &columnExtractDaoStub{getResp: nil, createErr: nil}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		err := repoImpl.UpsertColumnExtractConfig(context.Background(), &repo.UpsertColumnExtractConfigParam{
			WorkspaceId:  1,
			PlatformType: "coze_loop",
			SpanListType: "llm_span",
			AgentName:    "agent-1",
			Config:       `[{"Column":"input","JSONPath":"$.messages[0].content"}]`,
			UserID:       "u1",
		})
		assert.NoError(t, err)
		if assert.NotNil(t, stub.lastCreated) {
			assert.Equal(t, int64(100), stub.lastCreated.ID)
			assert.Equal(t, int64(1), stub.lastCreated.WorkspaceID)
			assert.Equal(t, "coze_loop", stub.lastCreated.PlatformType)
			assert.Equal(t, "llm_span", stub.lastCreated.SpanListType)
			assert.Equal(t, "agent-1", stub.lastCreated.AgentName)
			assert.Equal(t, `[{"Column":"input","JSONPath":"$.messages[0].content"}]`, *stub.lastCreated.Config)
			assert.Equal(t, "u1", stub.lastCreated.CreatedBy)
			assert.Equal(t, "u1", stub.lastCreated.UpdatedBy)
			assert.NotZero(t, stub.lastCreated.CreatedAt)
			assert.NotZero(t, stub.lastCreated.UpdatedAt)
		}
	}
	{
		existing := &model2.ObservabilityColumnExtractConfig{
			ID:           10,
			WorkspaceID:  2,
			PlatformType: "coze_loop",
			SpanListType: "llm_span",
			AgentName:    "agent-2",
			IsDeleted:    true,
		}
		stub := &columnExtractDaoStub{getResp: existing, updateErr: nil}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		err := repoImpl.UpsertColumnExtractConfig(context.Background(), &repo.UpsertColumnExtractConfigParam{
			WorkspaceId:  2,
			PlatformType: "coze_loop",
			SpanListType: "llm_span",
			AgentName:    "agent-2",
			Config:       `[{"Column":"output","JSONPath":"$.data"}]`,
			UserID:       "u2",
		})
		assert.NoError(t, err)
		if assert.NotNil(t, stub.lastUpdated) {
			assert.Equal(t, `[{"Column":"output","JSONPath":"$.data"}]`, *stub.lastUpdated.Config)
			assert.Equal(t, "u2", stub.lastUpdated.UpdatedBy)
			assert.False(t, stub.lastUpdated.IsDeleted)
			assert.NotZero(t, stub.lastUpdated.UpdatedAt)
		}
	}
	{
		stub := &columnExtractDaoStub{getErr: assert.AnError}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		err := repoImpl.UpsertColumnExtractConfig(context.Background(), &repo.UpsertColumnExtractConfigParam{
			WorkspaceId: 3, PlatformType: "coze_loop", SpanListType: "all_span", Config: "{}", UserID: "u",
		})
		assert.Error(t, err)
	}
	{
		stub := &columnExtractDaoStub{getResp: nil, createErr: errors.New("dup")}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		err := repoImpl.UpsertColumnExtractConfig(context.Background(), &repo.UpsertColumnExtractConfigParam{
			WorkspaceId: 4, PlatformType: "coze_loop", SpanListType: "all_span", Config: "{}", UserID: "u",
		})
		assert.Error(t, err)
	}
	{
		existing := &model2.ObservabilityColumnExtractConfig{ID: 10, WorkspaceID: 5}
		stub := &columnExtractDaoStub{getResp: existing, updateErr: errors.New("update err")}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		err := repoImpl.UpsertColumnExtractConfig(context.Background(), &repo.UpsertColumnExtractConfigParam{
			WorkspaceId: 5, PlatformType: "coze_loop", SpanListType: "all_span", Config: "{}", UserID: "u5",
		})
		assert.Error(t, err)
	}
}

func TestColumnExtractConfigRepoImpl_GetColumnExtractConfig(t *testing.T) {
	{
		stub := &columnExtractDaoStub{getResp: nil}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		got, err := repoImpl.GetColumnExtractConfig(context.Background(), repo.GetColumnExtractConfigParam{
			WorkspaceId: 1, PlatformType: "coze_loop", SpanListType: "llm_span",
		})
		assert.NoError(t, err)
		assert.Nil(t, got)
	}
	{
		config := `[{"Column":"input","JSONPath":"$.messages[0].content"}]`
		stub := &columnExtractDaoStub{getResp: &model2.ObservabilityColumnExtractConfig{
			ID: 11, WorkspaceID: 2, PlatformType: "coze_loop", SpanListType: "llm_span", AgentName: "agent-1", Config: &config,
		}}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		got, err := repoImpl.GetColumnExtractConfig(context.Background(), repo.GetColumnExtractConfigParam{
			WorkspaceId: 2, PlatformType: "coze_loop", SpanListType: "llm_span", AgentName: "agent-1",
		})
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			assert.Equal(t, int64(2), got.WorkspaceID)
			assert.Equal(t, "coze_loop", got.PlatformType)
			assert.Equal(t, "llm_span", got.SpanListType)
			assert.Equal(t, "agent-1", got.AgentName)
			assert.Len(t, got.Columns, 1)
			assert.Equal(t, "input", got.Columns[0].Column)
			assert.Equal(t, "$.messages[0].content", got.Columns[0].JSONPath)
		}
	}
	{
		stub := &columnExtractDaoStub{getErr: errors.New("db err")}
		repoImpl := NewColumnExtractConfigRepoImpl(stub, idGenStub{})
		got, err := repoImpl.GetColumnExtractConfig(context.Background(), repo.GetColumnExtractConfigParam{
			WorkspaceId: 9, PlatformType: "coze_loop", SpanListType: "all_span",
		})
		assert.Error(t, err)
		assert.Nil(t, got)
	}
}
