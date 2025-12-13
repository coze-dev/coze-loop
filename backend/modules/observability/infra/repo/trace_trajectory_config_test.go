// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	model2 "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/stretchr/testify/assert"
)

// 简易 ID 生成器 stub
type idGenStub struct{}

func (s idGenStub) GenMultiIDs(ctx context.Context, counts int) ([]int64, error) {
	return []int64{1, 2, 3}, nil
}

func (idGenStub) GenID(ctx context.Context) (int64, error) { return 100, nil }

// 轨迹配置 DAO stub（捕获入参便于断言）
type trajectoryDaoStub struct {
	getResp     *model2.ObservabilityTrajectoryConfig
	getErr      error
	createErr   error
	updateErr   error
	lastCreated *model2.ObservabilityTrajectoryConfig
	lastUpdated *model2.ObservabilityTrajectoryConfig
}

func (t *trajectoryDaoStub) GetTrajectoryConfig(ctx context.Context, workspaceID int64) (*model2.ObservabilityTrajectoryConfig, error) {
	return t.getResp, t.getErr
}

func (t *trajectoryDaoStub) UpdateTrajectoryConfig(ctx context.Context, po *model2.ObservabilityTrajectoryConfig) error {
	t.lastUpdated = po
	return t.updateErr
}

func (t *trajectoryDaoStub) CreateTrajectoryConfig(ctx context.Context, po *model2.ObservabilityTrajectoryConfig) error {
	t.lastCreated = po
	return t.createErr
}

func TestTraceRepoImpl_UpsertTrajectoryConfig(t *testing.T) {
	// 创建新配置
	{
		trajStub := &trajectoryDaoStub{getResp: nil, createErr: nil}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		err = repoImpl.UpsertTrajectoryConfig(context.Background(), &repo.UpsertTrajectoryConfigParam{WorkspaceId: 1, Filters: "{}", UserID: "u"})
		assert.NoError(t, err)
		if assert.NotNil(t, trajStub.lastCreated) {
			assert.Equal(t, int64(1), trajStub.lastCreated.WorkspaceID)
			assert.Equal(t, "{}", *trajStub.lastCreated.Filter)
			assert.Equal(t, "u", trajStub.lastCreated.CreatedBy)
			assert.Equal(t, "u", trajStub.lastCreated.UpdatedBy)
			assert.False(t, trajStub.lastCreated.IsDeleted)
			assert.NotZero(t, trajStub.lastCreated.CreatedAt)
			assert.NotZero(t, trajStub.lastCreated.UpdatedAt)
		}
	}
	// 更新已有配置
	{
		orig := &model2.ObservabilityTrajectoryConfig{ID: 10, WorkspaceID: 2, Filter: nil, IsDeleted: true}
		trajStub := &trajectoryDaoStub{getResp: orig, updateErr: nil}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		err = repoImpl.UpsertTrajectoryConfig(context.Background(), &repo.UpsertTrajectoryConfigParam{WorkspaceId: 2, Filters: "{\"a\":1}", UserID: "u2"})
		assert.NoError(t, err)
		if assert.NotNil(t, trajStub.lastUpdated) {
			assert.Equal(t, int64(2), trajStub.lastUpdated.WorkspaceID)
			assert.Equal(t, "{\"a\":1}", *trajStub.lastUpdated.Filter)
			assert.Equal(t, "u2", trajStub.lastUpdated.UpdatedBy)
			assert.False(t, trajStub.lastUpdated.IsDeleted)
			assert.NotZero(t, trajStub.lastUpdated.UpdatedAt)
		}
	}
	// GetTrajectoryConfig 返回错误
	{
		trajStub := &trajectoryDaoStub{getErr: assert.AnError}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		err = repoImpl.UpsertTrajectoryConfig(context.Background(), &repo.UpsertTrajectoryConfigParam{WorkspaceId: 3, Filters: "{}", UserID: "u"})
		assert.Error(t, err)
	}
	// CreateTrajectoryConfig 返回错误
	{
		trajStub := &trajectoryDaoStub{getResp: nil, createErr: errors.New("dup")}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		err = repoImpl.UpsertTrajectoryConfig(context.Background(), &repo.UpsertTrajectoryConfigParam{WorkspaceId: 4, Filters: "{}", UserID: "u"})
		assert.Error(t, err)
	}
	// UpdateTrajectoryConfig 返回错误
	{
		orig := &model2.ObservabilityTrajectoryConfig{ID: 10, WorkspaceID: 5}
		trajStub := &trajectoryDaoStub{getResp: orig, updateErr: errors.New("update err")}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		err = repoImpl.UpsertTrajectoryConfig(context.Background(), &repo.UpsertTrajectoryConfigParam{WorkspaceId: 5, Filters: "{}", UserID: "u5"})
		assert.Error(t, err)
	}
}

func TestTraceRepoImpl_GetTrajectoryConfig(t *testing.T) {
	// dao 返回 nil
	{
		trajStub := &trajectoryDaoStub{getResp: nil}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		got, err := repoImpl.GetTrajectoryConfig(context.Background(), repo.GetTrajectoryConfigParam{WorkspaceId: 1})
		assert.NoError(t, err)
		assert.Nil(t, got)
	}
	// dao 返回 po（Filter 为有效 JSON）
	{
		filter := "{}"
		trajStub := &trajectoryDaoStub{getResp: &model2.ObservabilityTrajectoryConfig{ID: 11, WorkspaceID: 2, Filter: &filter, CreatedAt: time.Now(), UpdatedAt: time.Now(), CreatedBy: "u", UpdatedBy: "u"}}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		got, err := repoImpl.GetTrajectoryConfig(context.Background(), repo.GetTrajectoryConfigParam{WorkspaceId: 2})
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			assert.Equal(t, int64(2), got.WorkspaceID)
			// 有效 JSON 会被解析为非 nil 的 Filter（空结构）
			assert.NotNil(t, got.Filter)
		}
	}
	// dao 返回 po（Filter 为无效 JSON）
	{
		filter := "not-json"
		trajStub := &trajectoryDaoStub{getResp: &model2.ObservabilityTrajectoryConfig{ID: 12, WorkspaceID: 3, Filter: &filter}}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		got, err := repoImpl.GetTrajectoryConfig(context.Background(), repo.GetTrajectoryConfigParam{WorkspaceId: 3})
		assert.NoError(t, err)
		if assert.NotNil(t, got) {
			// 无效 JSON 不会设置 Filter
			assert.Nil(t, got.Filter)
		}
	}
	// dao error
	{
		trajStub := &trajectoryDaoStub{getErr: errors.New("db err")}
		repoImpl, err := NewTraceRepoImpl(nil, nil, nil, nil, trajStub, idGenStub{})
		assert.NoError(t, err)
		got, err := repoImpl.GetTrajectoryConfig(context.Background(), repo.GetTrajectoryConfigParam{WorkspaceId: 9})
		assert.Error(t, err)
		assert.Nil(t, got)
	}
}
