// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ExptManualRetryFilterService checks materialized data, including Resulted items.
// It remains separate from the older result client contract.
//
//go:generate mockgen -destination=mocks/expt_result_filter_sync.go -package=mocks . ExptManualRetryFilterService
type ExptManualRetryFilterService interface {
	ReconcileManualRetryFilterBeforeFinish(context.Context, int64, int64, int64) (bool, error)
}

// Each refresh owns an independent holder for the existing execution item key.
// Callbacks already holding this key call only the private locked helpers.
func (e ExptResultServiceImpl) withFilterItemLock(ctx context.Context, exptID, itemID int64, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if e.resultPublishLocker == nil {
		if _, current := e.ExptItemResultRepo.(repo.IExptCurrentResultRepo); current {
			return errno.NewMQRetryErr("filter independent item locker missing")
		}
		return fn(ctx) // Compatibility for older unit-test doubles only.
	}
	locker := e.resultPublishLocker()
	if locker == nil {
		return errno.NewMQRetryErr("filter item locker missing")
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", exptID, itemID)
	locked, lockCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, time.Minute)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !locked {
		return errno.NewMQRetryErr("filter item lock busy")
	}
	defer cancel()
	defer func() {
		if _, err := locker.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "unlock filter item: %v", err)
		}
	}()
	if err := lockCtx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if err := fn(contexts.WithCtxWriteDB(lockCtx)); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if err := lockCtx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	return nil
}

func sortedFilterItems(ids []int64) []int64 {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	return slices.Compact(ids)
}

func (e ExptResultServiceImpl) filterCandidateItems(ctx context.Context, spaceID, exptID int64, ids []int64) ([]int64, error) {
	if len(ids) > 0 {
		return sortedFilterItems(ids), nil
	}
	// Only IDs escape this enumeration. Payloads are read again under each lock.
	items, _, err := e.ExptItemResultRepo.ListItemResultsByExptID(contexts.WithCtxWriteDB(ctx), exptID, spaceID, entity.Page{}, false)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item != nil {
			ids = append(ids, item.ItemID)
		}
	}
	return sortedFilterItems(ids), nil
}

func (e ExptResultServiceImpl) UpsertExptTurnResultFilter(ctx context.Context, spaceID, exptID int64, ids []int64) error {
	if spaceID == 0 || exptID == 0 {
		return fmt.Errorf("UpsertExptTurnResultFilter: invalid space_id or expt_id")
	}
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if e.resultPublishLocker == nil {
		if _, current := e.ExptItemResultRepo.(repo.IExptCurrentResultRepo); !current {
			return e.upsertExptTurnResultFilterLocked(ctx, spaceID, exptID, ids)
		}
	}
	ids, err := e.filterCandidateItems(ctx, spaceID, exptID, ids)
	if err != nil {
		return err
	}
	var firstErr error
	for _, item := range ids {
		if err := ctx.Err(); err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if err := e.withFilterItemLock(ctx, exptID, item, func(ctx context.Context) error {
			return e.upsertExptTurnResultFilterLocked(ctx, spaceID, exptID, []int64{item})
		}); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return errno.WrapMQRetryErr(ctxErr)
			}
			if firstErr == nil {
				firstErr = err
				logs.CtxWarn(ctx, "UpsertExptTurnResultFilter item failed, continuing batch, expt_id: %d, item_id: %d, err: %v", exptID, item, err)
			}
		}
	}
	return firstErr
}

func (e ExptResultServiceImpl) CompareExptTurnResultFilters(ctx context.Context, spaceID, exptID int64, ids []int64, retry int32) error {
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if e.resultPublishLocker == nil {
		if _, current := e.ExptItemResultRepo.(repo.IExptCurrentResultRepo); !current {
			return e.compareExptTurnResultFiltersLegacy(ctx, spaceID, exptID, ids, retry)
		}
	}
	ids, err := e.filterCandidateItems(ctx, spaceID, exptID, ids)
	if err != nil {
		return err
	}
	var firstErr error
	for _, item := range ids {
		if err := ctx.Err(); err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if err := e.withFilterItemLock(ctx, exptID, item, func(ctx context.Context) error {
			return e.compareCurrentItemFilterLocked(ctx, spaceID, exptID, item, retry)
		}); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return errno.WrapMQRetryErr(ctxErr)
			}
			if firstErr == nil {
				firstErr = err
				logs.CtxWarn(ctx, "CompareExptTurnResultFilters item failed, continuing batch, expt_id: %d, item_id: %d, err: %v", exptID, item, err)
			}
		}
	}
	return firstErr
}

func (e ExptResultServiceImpl) ReconcileManualRetryFilterBeforeFinish(ctx context.Context, space, expt, run int64) (bool, error) {
	scanner, ok := e.ExptItemResultRepo.(repo.IExptManualRetryFilterRepo)
	if !ok {
		return false, errno.NewMQRetryErr("manual retry filter enumeration missing")
	}
	ids := []int64{}
	cursor := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
		page, next, err := scanner.ScanManualRetryFilterItems(ctx, space, expt, run, cursor, 200)
		if err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
		ids = append(ids, page...)
		if next == 0 {
			break
		}
		if next <= cursor {
			return false, errno.NewMQRetryErr("filter enumeration cursor stalled")
		}
		cursor = next
	}
	for _, item := range sortedFilterItems(ids) {
		if err := e.withFilterItemLock(ctx, expt, item, func(ctx context.Context) error {
			if e.resultPublishRepo == nil || e.exptTurnResultFilterRepo == nil {
				return errno.NewMQRetryErr("filter calibration dependencies missing")
			}
			// Publication's source reader accepts Resulted and finished runs. Execution's
			// active-only source reader would silently lose precisely these candidates.
			source, err := e.resultPublishRepo.GetItemResultPublishSource(ctx, space, expt, run, item)
			if err != nil {
				return err
			}
			if source == nil {
				return errno.NewMQRetryErr("filter calibration current binding unavailable")
			}
			manual, err := entity.ParseManualRetryContext(source.Item.Ext)
			if err != nil {
				return err
			}
			if manual == nil || manual.RunID != run || manual.RequestID != source.Source.Current.RequestID {
				return errno.NewMQRetryErr("filter calibration request changed")
			}
			if !entity.IsItemRunFinished(source.Item.Status) || !entity.IsItemRunFinished(entity.ItemRunState(source.ItemLog.Status)) || source.ItemLog.ResultState != int32(entity.ExptItemResultStateResulted) {
				return errno.NewMQRetryErr("filter calibration publication pending")
			}
			filters, err := e.buildCurrentItemFilterLocked(ctx, space, expt, item)
			if err != nil {
				return err
			}
			if len(filters) == 0 {
				return errno.NewMQRetryErr("filter calibration source turns missing")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := e.exptTurnResultFilterRepo.Save(ctx, filters); err != nil {
				return err
			}
			return ctx.Err()
		}); err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
	}
	return true, nil
}

func (e ExptResultServiceImpl) materializedFilterMatchesLocked(ctx context.Context, space, expt, item int64) (bool, error) {
	filters, err := e.buildCurrentItemFilterLocked(ctx, space, expt, item)
	if err != nil {
		return false, err
	}
	return e.compareMaterializedFilterLocked(ctx, space, expt, item, filters)
}

func (e ExptResultServiceImpl) compareMaterializedFilterLocked(ctx context.Context, space, expt, item int64, expected []*entity.ExptTurnResultFilterEntity) (bool, error) {
	if len(expected) == 0 {
		return false, nil
	}
	actual, fullRead, err := e.readMaterializedItemFilterLocked(ctx, space, expt, item, expected[0].CreatedDate)
	if err != nil {
		return false, err
	}
	if len(actual) != len(expected) {
		return false, nil
	}
	byTurn := e.createTurnKeyToFilterMap(actual)
	compareItemVersion := true
	if storage, ok := e.exptTurnResultFilterRepo.(repo.IExptMaterializedItemVersionRepo); ok {
		compareItemVersion = storage.PersistsMaterializedItemVersion()
	}
	for _, want := range expected {
		got := byTurn[GenerateTurnKey(space, expt, item, want.TurnID)]
		// Both repositories' current compare contract returns these filter fields.
		// No future BMQ source guard or ingestion ordering is implied by equality.
		if got == nil || got.ItemIdx != want.ItemIdx || got.Status != want.Status || got.EvaluatorScoreCorrected != want.EvaluatorScoreCorrected || !maps.Equal(got.EvaluatorScore, want.EvaluatorScore) || got.EvalTargetData["actual_output"] != want.EvalTargetData["actual_output"] {
			return false, nil
		}
		if fullRead && (compareItemVersion && got.ItemVersionID != want.ItemVersionID || got.EvalSetID != want.EvalSetID || got.EvalSetVersionID != want.EvalSetVersionID || !maps.Equal(got.EvalTargetData, want.EvalTargetData) || !maps.Equal(got.AnnotationFloat, want.AnnotationFloat) || !maps.Equal(got.AnnotationBool, want.AnnotationBool) || !maps.Equal(got.AnnotationString, want.AnnotationString) || !maps.Equal(got.EvalTargetMetrics, want.EvalTargetMetrics) || !filterWeightedScoreEqual(got.EvaluatorWeightedScore, want.EvaluatorWeightedScore)) {
			return false, nil
		}
	}
	if !fullRead {
		return false, nil
	} // Existing async readers cannot prove complete projection equality.
	return true, ctx.Err()
}

func (e ExptResultServiceImpl) buildCurrentItemFilterLocked(ctx context.Context, space, expt, item int64) ([]*entity.ExptTurnResultFilterEntity, error) {
	return e.buildExptTurnResultFiltersLocked(contexts.WithCtxWriteDB(ctx), space, expt, []int64{item})
}

// The existing CK schema serializes a nil weighted score as non-null zero.
// Per-evaluator maps retain key presence, so missing scores never become 0.
func filterWeightedScoreEqual(got, want *float64) bool {
	if want == nil {
		return got == nil || *got == 0
	}
	return reflect.DeepEqual(got, want)
}

// Called only by local paths whose execution mutex still encloses the call.
func refreshFilterWithHeldItemLock(ctx context.Context, svc ExptResultService, space, expt, item int64) error {
	if local, ok := svc.(*ExptResultServiceImpl); ok {
		return local.upsertExptTurnResultFilterLocked(contexts.WithCtxWriteDB(ctx), space, expt, []int64{item})
	}
	return svc.UpsertExptTurnResultFilter(ctx, space, expt, []int64{item})
}

// Recover only the final calibration boundary so runtime failures cannot reach
// the scheduler's CompleteRun/Failed fallback.
func reconcileManualRetryFilterBeforeFinish(ctx context.Context, service ExptManualRetryFilterService, space, expt, run int64) (ready bool, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			ready = false
			err = errno.NewMQRetryErr(fmt.Sprintf("filter calibration runtime failure: %v", failure))
		}
	}()
	return service.ReconcileManualRetryFilterBeforeFinish(ctx, space, expt, run)
}

func (e ExptResultServiceImpl) readMaterializedItemFilterLocked(ctx context.Context, space, expt, item int64, date time.Time) ([]*entity.ExptTurnResultFilterEntity, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	started := time.Now()
	full, complete := e.exptTurnResultFilterRepo.(repo.IExptMaterializedFilterRepo)
	var rows []*entity.ExptTurnResultFilterEntity
	var err error
	if complete {
		rows, err = full.GetMaterializedByExptIDItemIDs(ctx, strconv.FormatInt(space, 10), strconv.FormatInt(expt, 10), date.Format("2006-01-02"), []string{strconv.FormatInt(item, 10)})
	} else {
		rows, err = e.exptTurnResultFilterRepo.GetByExptIDItemIDs(ctx, strconv.FormatInt(space, 10), strconv.FormatInt(expt, 10), date.Format("2006-01-02"), []string{strconv.FormatInt(item, 10)})
	}
	if e.Metric != nil {
		e.Metric.EmitExptTurnResultFilterQueryLatency(space, started.Unix(), err != nil)
	}
	return rows, complete, err
}

func (e ExptResultServiceImpl) compareCurrentItemFilterLocked(ctx context.Context, space, expt, item int64, retry int32) error {
	expected, err := e.buildCurrentItemFilterLocked(ctx, space, expt, item)
	if err != nil {
		return err
	}
	if len(expected) == 0 || expected[0].CreatedDate.IsZero() {
		return nil
	}
	actual, _, err := e.readMaterializedItemFilterLocked(ctx, space, expt, item, expected[0].CreatedDate)
	if err != nil {
		return err
	}
	byTurn := e.createTurnKeyToFilterMap(actual)
	for _, want := range expected {
		got := byTurn[GenerateTurnKey(space, expt, item, want.TurnID)]
		missing := got == nil
		scoreDiff, outputDiff, diff := false, false, missing
		if !missing {
			scoreDiff = !maps.Equal(want.EvaluatorScore, got.EvaluatorScore)
			outputDiff = want.EvalTargetData["actual_output"] != got.EvalTargetData["actual_output"]
			diff = scoreDiff || outputDiff || want.Status != got.Status || want.ItemIdx != got.ItemIdx || want.EvaluatorScoreCorrected != got.EvaluatorScoreCorrected
		}
		if !diff || retry >= 3 {
			if e.Metric != nil {
				e.Metric.EmitExptTurnResultFilterCheck(space, scoreDiff, outputDiff, diff, missing)
			}
			if diff {
				logs.CtxError(ctx, "CompareExptTurnResultFilters finish, diff exist, retryTimes >= maxRetryTimes, itemID: %d, turnID: %d, retryTimes: %d", item, want.TurnID, retry)
			}
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := e.publisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{ExperimentID: expt, SpaceID: space, ItemID: []int64{item}, RetryTimes: gptr.Of(retry + 1), FilterType: gptr.Of(entity.UpsertExptTurnResultFilterTypeCheck)}, gptr.Of(10*time.Second)); err != nil {
			return err
		}
	}
	return nil
}
