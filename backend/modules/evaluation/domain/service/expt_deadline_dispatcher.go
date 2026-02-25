// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	defaultDeadlineScanInterval = 45 * time.Second
	defaultDeadlineBatchLimit   = 200
	defaultDeadlineClaimTTL     = 60 * time.Second // 抢占锁 TTL，未完成则过期供其他实例重试
)

// ExptDeadlineDispatcher 轮询 ZSET 到期成员，从 DB 组事件后派发
type ExptDeadlineDispatcher struct {
	store      repo.IExptDeadlineStore
	publisher  events.ExptEventPublisher
	manager    IExptManager
	interval   time.Duration
	batchLimit int
}

func NewExptDeadlineDispatcher(store repo.IExptDeadlineStore, publisher events.ExptEventPublisher, manager IExptManager) *ExptDeadlineDispatcher {
	return &ExptDeadlineDispatcher{
		store:      store,
		publisher:  publisher,
		manager:    manager,
		interval:   defaultDeadlineScanInterval,
		batchLimit: defaultDeadlineBatchLimit,
	}
}

func (d *ExptDeadlineDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		d.runOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *ExptDeadlineDispatcher) runOnce(ctx context.Context) {
	now := time.Now().Unix()
	members, err := d.store.ScanDue(ctx, now, d.batchLimit)
	if err != nil {
		logs.CtxWarn(ctx, "expt deadline ScanDue failed, err: %v", err)
		return
	}
	for _, member := range members {
		ok, err := d.store.TryClaim(ctx, member, defaultDeadlineClaimTTL)
		if err != nil || !ok {
			continue // 其他实例已抢占或 Redis 异常，跳过
		}
		var spaceID, exptID, runID int64
		if _, err := fmt.Sscanf(member, "expt:%d:%d:%d", &spaceID, &exptID, &runID); err != nil {
			logs.CtxWarn(ctx, "expt deadline invalid member: %s", member)
			_ = d.store.Remove(ctx, member)
			continue
		}
		session := &entity.Session{}
		runLog, err := d.manager.GetRunLog(ctx, exptID, runID, spaceID, session)
		if err != nil || runLog == nil {
			logs.CtxWarn(ctx, "expt deadline GetRunLog failed, member: %s, err: %v", member, err)
			_ = d.store.Remove(ctx, member)
			continue
		}
		session.UserID = runLog.CreatedBy
		expt, err := d.manager.GetDetail(ctx, exptID, spaceID, session)
		if err != nil || expt == nil {
			logs.CtxWarn(ctx, "expt deadline GetDetail failed, member: %s, err: %v", member, err)
			continue
		}
		event := &entity.ExptScheduleEvent{
			SpaceID:     spaceID,
			ExptID:      exptID,
			ExptRunID:   runID,
			ExptRunMode: entity.ExptRunMode(runLog.Mode),
			ExptType:    expt.ExptType,
			CreatedAt:   time.Now().Unix(),
			Session:     session,
		}
		if err = d.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(time.Second*3)); err != nil {
			logs.CtxWarn(ctx, "expt deadline Publish failed, member: %s, err: %v", member, err)
			continue
		}
		_ = d.store.Remove(ctx, member)
	}
}
