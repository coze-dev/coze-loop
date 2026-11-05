// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

var ErrExptFilterFutureTimestamp = errors.New("future_timestamp")

func copyFilterAccelerator(src *entity.ExptTurnResultFilterAccelerator) *entity.ExptTurnResultFilterAccelerator {
	copy := *src
	if src.MapCond != nil {
		value := *src.MapCond
		copy.MapCond = &value
	}
	if src.ItemSnapshotCond != nil {
		value := *src.ItemSnapshotCond
		copy.ItemSnapshotCond = &value
	}
	if src.KeywordSearch != nil {
		value := *src.KeywordSearch
		copy.KeywordSearch = &value
		if value.ItemSnapshotFilter != nil {
			filter := *value.ItemSnapshotFilter
			copy.KeywordSearch.ItemSnapshotFilter = &filter
		}
	}
	return &copy
}

func (e ExptResultServiceImpl) freezeFilterPlan(ctx context.Context, ids []int64) (context.Context, error) {
	if contexts.ExptFilterPlanFromContext(ctx) != nil {
		return ctx, nil
	}
	index, ok := e.exptTurnResultFilterRepo.(repo.IExptFilterIndexRepo)
	if !ok {
		return ctx, nil
	}
	plan, err := index.SnapshotPlan(ctx, sortedFilterItems(ids))
	if err != nil {
		return ctx, err
	}
	if plan == nil || len(plan.RequiredTargets) == 0 {
		return ctx, fmt.Errorf("config_unavailable: empty filter plan")
	}
	return contexts.WithExptFilterPlan(ctx, plan), nil
}

func (e ExptResultServiceImpl) saveCurrentFilterIndex(ctx context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
	index, ok := e.exptTurnResultFilterRepo.(repo.IExptFilterIndexRepo)
	if !ok {
		return e.exptTurnResultFilterRepo.Save(ctx, rows)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.ExptID)
		}
	}
	ctx, err := e.freezeFilterPlan(ctx, ids)
	if err != nil {
		return err
	}
	return index.SaveIndex(ctx, rows, contexts.ExptFilterPlanFromContext(ctx), entity.ExptFilterCurrent)
}

// CompareExptFilterRows validates one sink before comparing its full business projection.
func CompareExptFilterRows(expected []*entity.ExptTurnResultFilterEntity, actual []*entity.ExptFilterRow, target entity.ExptFilterTarget, now time.Time) (bool, error) {
	key := func(row *entity.ExptTurnResultFilterEntity) entity.ExptFilterKey {
		return entity.ExptFilterKey{SpaceID: row.SpaceID, ExptID: row.ExptID, ItemID: row.ItemID, TurnID: row.TurnID}
	}
	wants := make(map[entity.ExptFilterKey]*entity.ExptTurnResultFilterEntity, len(expected))
	for _, row := range expected {
		if row == nil {
			return false, fmt.Errorf("invalid_projection: nil source row")
		}
		k := key(row)
		if _, exists := wants[k]; exists {
			return false, fmt.Errorf("invalid_projection: duplicate source key %+v", k)
		}
		wants[k] = row
	}
	seen := make(map[entity.ExptFilterKey]struct{}, len(actual))
	match := len(expected) == len(actual)
	for _, row := range actual {
		if row == nil || row.Filter == nil {
			return false, fmt.Errorf("invalid_projection: nil stored row")
		}
		k := key(row.Filter)
		if _, exists := seen[k]; exists {
			return false, fmt.Errorf("invalid_projection: duplicate stored key %+v", k)
		}
		seen[k] = struct{}{}
		if target == entity.ExptFilterTargetV2 {
			if row.WriteTimestampMS == nil {
				return false, fmt.Errorf("invalid_projection: missing write timestamp for %+v", k)
			}
			if *row.WriteTimestampMS > uint64(now.UnixMilli()) {
				return false, fmt.Errorf("%w: key %+v", ErrExptFilterFutureTimestamp, k)
			}
		}
		if !entity.ExptFilterProjectionEqual(wants[k], row.Filter) {
			match = false
		}
	}
	return match, nil
}

func (e ExptResultServiceImpl) compareFilterIndex(ctx context.Context, index repo.IExptFilterIndexRepo, space, expt, item int64, expected []*entity.ExptTurnResultFilterEntity) (bool, error) {
	if len(expected) == 0 {
		return false, fmt.Errorf("source_unknown: empty projection")
	}
	ctx, err := e.freezeFilterPlan(ctx, []int64{expt})
	if err != nil {
		return false, err
	}
	match := true
	for _, target := range contexts.ExptFilterPlanFromContext(ctx).RequiredTargets {
		rows, err := index.ReadIndex(ctx, target, entity.ExptFilterReadQuery{Scope: entity.ExptFilterScope{SpaceID: space, ExptID: expt}, CreatedDate: expected[0].CreatedDate.Format("2006-01-02"), ItemIDs: []int64{item}})
		if err != nil {
			return false, err
		}
		equal, err := CompareExptFilterRows(expected, rows, target, time.Now())
		if err != nil {
			return false, err
		}
		match = match && equal
	}
	return match, nil
}

// Empty scopes are enumerated a page at a time; only one page of IDs is retained.
func (e ExptResultServiceImpl) forEachFilterItemPage(ctx context.Context, space, expt int64, ids []int64, fn func([]int64) error) error {
	const batch = 100
	if len(ids) > 0 {
		ids = sortedFilterItems(ids)
		for start := 0; start < len(ids); start += batch {
			if err := fn(ids[start:min(start+batch, len(ids))]); err != nil {
				return err
			}
		}
		return nil
	}
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, _, err := e.ExptItemResultRepo.ListItemResultsByExptID(contexts.WithCtxWriteDB(ctx), expt, space, entity.NewPage(page, batch), false)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		items := make([]int64, 0, len(rows))
		for _, row := range rows {
			if row == nil || row.ItemID <= 0 {
				return fmt.Errorf("source_unknown: invalid item in scope %d/%d", space, expt)
			}
			items = append(items, row.ItemID)
		}
		if err := fn(sortedFilterItems(items)); err != nil {
			return err
		}
		if len(rows) < batch {
			return nil
		}
	}
}
