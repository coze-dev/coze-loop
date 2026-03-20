// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type snapshotContext struct {
	spaceID       int64
	versionID     int64
	nowRetryTimes int64
	isFinished    bool
	version       *entity.DatasetVersion
}

func (s *DatasetServiceImpl) RunSnapshotItemJob(ctx context.Context, msg *entity.JobRunMessage) error {
	if msg == nil || msg.Type != entity.DatasetSnapshotJob {
		logs.CtxWarn(ctx, "message is invalid, msg=%v", msg)
		return nil
	}
	processCtx, err := s.initProgressCtx(ctx, msg)
	if err != nil {
		return err
	}
	if processCtx.version.SnapshotStatus.IsFinished() {
		logs.CtxInfo(ctx, "snapshot of version is finished, skip. id=%d, status=%s", processCtx.version.ID, processCtx.version.SnapshotStatus)
		return nil
	}
	ok, err := s.checkRetryable(ctx, processCtx)
	if err != nil {
		return err
	}
	if !ok {
		logs.CtxInfo(ctx, "snapshot version %d failed for exceed max_retry_time", processCtx.versionID)
		return nil
	}
	if err := s.process(ctx, processCtx); err != nil {
		return s.handleErr(ctx, err, processCtx, msg)
	}
	return nil
}

func (s *DatasetServiceImpl) process(ctx context.Context, processCtx *snapshotContext) error {
	// 1. lock version
	key := FormatVersionSnapshottingKey(processCtx.versionID)
	ok, ctx, cancel, err := s.locker.LockBackoffWithRenew(ctx, key, s.retryCfg().GetMaxProcessingTime(), 20*time.Minute)
	if err != nil {
		return errno.NewRetryableErr(err)
	}
	logs.CtxInfo(ctx, "version %d locked, snapshot start", processCtx.versionID)
	defer cancel()
	if !ok {
		logs.CtxWarn(ctx, "version has been locked by another handler, skip consume. version_id=%d")
		return nil
	}
	// 2. loop scan and upsert items
	for !processCtx.isFinished {
		select {
		case <-ctx.Done():
			// context timeout or has been canceled
			return errno.NewRetryableErr(errors.Errorf("snapshot paused due to context timeout or has been canceled"))
		default:
			hasMore, err := s.scanAndUpsertItems(ctx, processCtx.version, processCtx.version.SnapshotProgress)
			if err != nil {
				return errno.NewRetryableErr(err)
			}
			if err := s.commitProgress(ctx, processCtx, hasMore); err != nil {
				if bizErr, ok := kerrors.FromBizStatusError(err); ok && bizErr.BizStatusCode() == errno.ConcurrentDatasetOperationsCode {
					processCtx.isFinished = true
					logs.CtxInfo(ctx, "update version %d snapshot with concurrent, skip, and this progress will finish", processCtx.versionID)
					return nil
				}
				return errno.NewRetryableErr(err)
			}
		}
	}
	return nil
}

func (s *DatasetServiceImpl) scanAndUpsertItems(ctx context.Context, version *entity.DatasetVersion, progress *entity.SnapshotProgress) (bool, error) {
	const defaultPageSize = 50
	query := repo.NewListItemsParamsFromVersion(version, func(q *repo.ListItemsParams) {
		q.Paginator = pagination.New(pagination.WithCursor(progress.Cursor), pagination.WithLimit(defaultPageSize))
	})
	items, pageRes, err := s.repo.ListItems(ctx, query)
	if err != nil {
		return false, err
	}
	progress.Cursor = pageRes.Cursor
	snapshots := gslice.Map(items, func(item *entity.Item) *entity.ItemSnapshot {
		return &entity.ItemSnapshot{
			VersionID: version.ID,
			Snapshot:  item,
			CreatedAt: time.Now(),
		}
	})
	if _, err := s.repo.BatchUpsertItemSnapshots(ctx, snapshots); err != nil {
		return false, err
	}
	return pageRes.Cursor != "", nil
}

func (s *DatasetServiceImpl) commitProgress(ctx context.Context, processCtx *snapshotContext, hasMore bool) error {
	if !hasMore {
		if err := s.commitToCompleted(ctx, processCtx); err != nil {
			return err
		}
		return nil
	}
	if processCtx.version.SnapshotStatus == entity.SnapshotStatusUnstarted {
		if err := s.commitToInProgress(ctx, processCtx); err != nil {
			return err
		}
	}
	return nil
}

func (s *DatasetServiceImpl) commitToCompleted(ctx context.Context, processCtx *snapshotContext) error {
	// count total from db to avoid inconsistency
	total, err := s.repo.CountItemSnapshots(ctx, &repo.ListItemSnapshotsParams{SpaceID: processCtx.spaceID, VersionID: processCtx.versionID}, repo.WithMaster())
	if err != nil {
		return err
	}
	// update snapshot status to completed
	patch := buildUpdateVersionPatch(processCtx, entity.SnapshotStatusCompleted, func(v *entity.DatasetVersion) {
		v.ItemCount = total
	})
	if err := s.repo.PatchVersion(ctx, patch, buildUpdateVersionWhere(processCtx)); err != nil {
		return err
	}
	processCtx.isFinished = true
	processCtx.version.SnapshotStatus = entity.SnapshotStatusCompleted
	processCtx.version.UpdateVersion = processCtx.version.UpdateVersion + 1
	return nil
}

func (s *DatasetServiceImpl) commitToInProgress(ctx context.Context, processCtx *snapshotContext) error {
	if err := s.repo.PatchVersion(ctx, buildUpdateVersionPatch(processCtx, entity.SnapshotStatusInProgress),
		buildUpdateVersionWhere(processCtx)); err != nil {
		return err
	}
	processCtx.version.SnapshotStatus = entity.SnapshotStatusInProgress
	processCtx.version.UpdateVersion = processCtx.version.UpdateVersion + 1
	return nil
}

func (s *DatasetServiceImpl) commitToFailed(ctx context.Context, processCtx *snapshotContext) error {
	if err := s.repo.PatchVersion(ctx, buildUpdateVersionPatch(processCtx, entity.SnapshotStatusFailed),
		buildUpdateVersionWhere(processCtx)); err != nil {
		return err
	}
	processCtx.isFinished = true
	processCtx.version.SnapshotStatus = entity.SnapshotStatusFailed
	processCtx.version.UpdateVersion = processCtx.version.UpdateVersion + 1
	return nil
}

func (s *DatasetServiceImpl) checkRetryable(ctx context.Context, processCtx *snapshotContext) (bool, error) {
	if processCtx.nowRetryTimes > s.retryCfg().MaxRetryTimes {
		// exceed max retry time, commit to failed
		if err := s.commitToFailed(ctx, processCtx); err != nil {
			return false, errno.NewRetryableErr(err)
		}
		return false, nil
	}
	return true, nil
}

func (s *DatasetServiceImpl) initProgressCtx(ctx context.Context, msg *entity.JobRunMessage) (*snapshotContext, error) {
	versionID, err := s.getVersionIDFromMsg(msg)
	if err != nil {
		return nil, err
	}
	version, err := s.repo.GetVersion(ctx, msg.SpaceID, versionID)
	if err != nil {
		return nil, errno.NewRetryableErr(err)
	}
	nowRetryTimes, err := s.getRetryTimeFromMsg(msg)
	if err != nil {
		return nil, err
	}
	if version.SnapshotProgress == nil {
		version.SnapshotProgress = new(entity.SnapshotProgress)
	}
	return &snapshotContext{
		spaceID:       version.SpaceID,
		versionID:     versionID,
		nowRetryTimes: nowRetryTimes,
		isFinished:    false,
		version:       version,
	}, nil
}

func (s *DatasetServiceImpl) getVersionIDFromMsg(msg *entity.JobRunMessage) (int64, error) {
	if msg == nil {
		return 0, errors.Errorf("msg is nil")
	}
	versionID, err := conv.Int64(msg.Extra["version_id"])
	if err != nil || versionID == 0 {
		return 0, errors.Errorf("version_id invalid, err=%v, msg=%v", err, msg)
	}
	return versionID, nil
}

func (s *DatasetServiceImpl) getRetryTimeFromMsg(msg *entity.JobRunMessage) (int64, error) {
	if msg == nil {
		return 0, errors.Errorf("msg is nil")
	}
	retryTimes, ok := msg.Extra["retry_times"]
	if !ok {
		return 0, nil
	}
	retryTimesI, err := conv.Int64(retryTimes)
	if err != nil {
		return 0, errors.Errorf("retry_times invalid, err=%v, msg=%v", err, msg)
	}
	return retryTimesI, nil
}

func (s *DatasetServiceImpl) handleErr(ctx context.Context, err error, processCtx *snapshotContext, msg *entity.JobRunMessage) error {
	if err == nil {
		return nil
	}
	if errno.IsRetryableErr(err) {
		// exceed max retry time, commit to failed
		processCtx.nowRetryTimes += 1
		if ok, err := s.checkRetryable(ctx, processCtx); err != nil || !ok {
			return err
		}
		// send retry message

		msg := &entity.JobRunMessage{
			Type:    entity.DatasetSnapshotJob,
			SpaceID: processCtx.spaceID,
			Extra: map[string]string{
				"version_id":  fmt.Sprintf("%d", processCtx.versionID),
				"retry_times": fmt.Sprintf("%d", processCtx.nowRetryTimes),
			},
			Operator: msg.Operator,
		}
		err = s.producer.Send(ctx, msg, []mq.MessageOpt{
			mq.WithKey((fmt.Sprintf("%d", processCtx.versionID))),
			mq.WithDelayTimeLevel(s.retryCfg().GetRetryInterval()),
		}...)
		if err != nil {
			logs.CtxError(ctx, "send retry snapshot msg failed, version_id:%d, err:%v", processCtx.versionID, err)
			return err
		}
		return nil
	}
	logs.CtxError(ctx, "snapshot version %d failed for un-retryable err, err=%v", processCtx.versionID, err)
	return nil
}

func buildUpdateVersionPatch(processCtx *snapshotContext, status entity.SnapshotStatus, opts ...func(v *entity.DatasetVersion)) *entity.DatasetVersion {
	patch := &entity.DatasetVersion{
		SnapshotStatus:   status,
		SnapshotProgress: processCtx.version.SnapshotProgress,
		UpdateVersion:    processCtx.version.UpdateVersion + 1,
	}
	for _, opt := range opts {
		opt(patch)
	}
	return patch
}

func buildUpdateVersionWhere(processCtx *snapshotContext) *entity.DatasetVersion {
	return &entity.DatasetVersion{
		ID:            processCtx.versionID,
		SpaceID:       processCtx.spaceID,
		UpdateVersion: processCtx.version.UpdateVersion,
	}
}

var VersionSnapshottingKey = `version:%d:snapshotting` // version:{version_id}:snapshotting, 版本快照锁

func FormatVersionSnapshottingKey(versionID int64) string {
	return fmt.Sprintf(VersionSnapshottingKey, versionID)
}
