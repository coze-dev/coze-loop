// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const CacheKeyObjListWithTask = "ObjListWithTask"

// TaskCacheInfo represents task cache information
type TaskCacheInfo struct {
	WorkspaceIDs []string
	BotIDs       []string
	Tasks        []*entity.ObservabilityTask
	UpdateTime   time.Time
}

type LocalCache struct {
	taskCache sync.Map
}

func NewLocalCache() *LocalCache {
	return &LocalCache{}
}

func (l *LocalCache) StoneTaskCache(info TaskCacheInfo) {
	l.taskCache.Store(CacheKeyObjListWithTask, info)
}

func (l *LocalCache) LoadTaskCache(ctx context.Context) TaskCacheInfo {
	// First, try to retrieve tasks from cache
	objListWithTask, ok := l.taskCache.Load(CacheKeyObjListWithTask)
	if !ok {
		// Cache is empty, fallback to the database
		logs.CtxError(ctx, "Cache is empty, retrieving task list from database")
		return TaskCacheInfo{}
	}

	cacheInfo, ok := objListWithTask.(TaskCacheInfo)
	if !ok {
		logs.CtxError(ctx, "Cache data type mismatch")
		return TaskCacheInfo{}
	}

	logs.CtxInfo(ctx, "Retrieve task list from cache, taskCount=%d, spaceCount=%d, botCount=%d", len(cacheInfo.Tasks), len(cacheInfo.WorkspaceIDs), len(cacheInfo.BotIDs))
	return cacheInfo
}
