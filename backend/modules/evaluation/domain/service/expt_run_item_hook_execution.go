// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"maps"
	"sync"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
)

type itemHookExecutionKey struct{}
type itemHookExecutionCheck func(context.Context) error

func checkItemHookExecution(ctx context.Context) error {
	check, _ := ctx.Value(itemHookExecutionKey{}).(itemHookExecutionCheck)
	if check == nil {
		return nil
	}
	if ctx.Err() != nil {
		return itemHookControlError{wait: true}
	}
	return check(ctx)
}

// Split every branch: errors.As alone would mistake mixed failures for control stops.
func splitItemHookControl(err error) (control bool, failure error) {
	if err == nil {
		return false, nil
	}
	if _, ok := err.(itemHookControlError); ok {
		return true, nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var failures []error
		for _, child := range joined.Unwrap() {
			found, rest := splitItemHookControl(child)
			control = control || found
			if rest != nil {
				failures = append(failures, rest)
			}
		}
		if control {
			return true, errors.Join(failures...)
		}
	} else if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if found, rest := splitItemHookControl(wrapped.Unwrap()); found {
			return true, rest
		}
	}
	return false, err
}

func itemHookControlOnly(err error) bool {
	control, failure := splitItemHookControl(err)
	return control && failure == nil
}

// The pool can cancel a queued task before its guarded closure is entered.
func itemHookPoolError(ctx context.Context, err error) error {
	if err == nil || ctx.Err() == nil || ctx.Value(itemHookExecutionKey{}) == nil {
		return err
	}
	if err == ctx.Err() {
		return itemHookControlError{wait: true}
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		mapped := make([]error, 0, len(children))
		for _, child := range children {
			mapped = append(mapped, itemHookPoolError(ctx, child))
		}
		return errors.Join(mapped...)
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok && errors.Is(err, ctx.Err()) {
		return itemHookPoolError(ctx, wrapped.Unwrap())
	}
	return err
}

// Exec returns only its first error, even when another dispatched worker failed.
type itemHookTaskErrors struct {
	mu   sync.Mutex
	errs []error
}

func (c *itemHookTaskErrors) capture(ctx context.Context, task func() error) func() error {
	if ctx.Value(itemHookExecutionKey{}) == nil {
		return task
	}
	return func() (err error) {
		defer func() {
			if err != nil {
				c.mu.Lock()
				c.errs = append(c.errs, err)
				c.mu.Unlock()
			}
		}()
		defer goroutine.Recover(ctx, &err)
		return task()
	}
}

func (c *itemHookTaskErrors) result(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.errs) == 0 {
		return err
	}
	all := append([]error{err}, c.errs...)
	for _, candidate := range all {
		control, _ := splitItemHookControl(candidate)
		if control || errors.Is(candidate, context.Canceled) || errors.Is(candidate, context.DeadlineExceeded) {
			return errors.Join(all...)
		}
	}
	return err
}

func mergeItemHookEvaluatorRefs(old *entity.EvaluatorResults, records []*entity.EvaluatorRecord) *entity.EvaluatorResults {
	merged := &entity.EvaluatorResults{}
	if old != nil {
		merged.EvalVerIDToResID = maps.Clone(old.EvalVerIDToResID)
		for _, ref := range old.Registered {
			if ref != nil {
				copy := *ref
				merged.Registered = append(merged.Registered, &copy)
			}
		}
		for _, ref := range old.Inline {
			if ref != nil {
				copy := *ref
				merged.Inline = append(merged.Inline, &copy)
			}
		}
	}
	upsert := func(versionID int64, alias string, recordID int64) {
		for _, ref := range merged.Registered {
			if ref.VersionID == versionID && ref.Alias == alias {
				ref.RecordID = recordID
				return
			}
		}
		merged.Registered = append(merged.Registered, &entity.RegisteredEvalResult{VersionID: versionID, Alias: alias, RecordID: recordID})
	}
	// Readers preferring the new format must still see legacy map-only references.
	for versionID, recordID := range merged.EvalVerIDToResID {
		found := false
		for _, ref := range merged.Registered {
			found = found || (ref.VersionID == versionID && ref.Alias == "")
		}
		if !found {
			upsert(versionID, "", recordID)
		}
	}
	for _, record := range records {
		if record == nil || record.ID <= 0 {
			continue
		}
		if record.SourceType == entity.EvaluatorRecordSourceTypeInline {
			found := false
			for _, ref := range merged.Inline {
				if ref.InlineKey == record.InlineKey {
					ref.RecordID, found = record.ID, true
					break
				}
			}
			if !found {
				merged.Inline = append(merged.Inline, &entity.InlineEvalResult{InlineKey: record.InlineKey, RecordID: record.ID})
			}
			continue
		}
		upsert(record.EvaluatorVersionID, record.Alias, record.ID)
		if record.Alias == "" && merged.EvalVerIDToResID != nil {
			merged.EvalVerIDToResID[record.EvaluatorVersionID] = record.ID
		}
	}
	return merged
}
