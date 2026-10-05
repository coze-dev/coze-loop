// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func hookTerminationStatus(status entity.ExptStatus) bool {
	return status == entity.ExptStatus_Terminated || status == entity.ExptStatus_SystemTerminated
}

func (e *ExptMangerImpl) setHookTerminating(ctx context.Context, key entity.HookRunKey) (bool, error) {
	if e.finalization == nil {
		return false, nil
	}
	if (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return true, entity.ErrHookStoreCorrupt
	}
	source, _, err := e.hookNormalCompletionInput(ctx, key)
	if err != nil {
		return true, err
	}
	if !source.Managed {
		return false, nil
	}
	_, err = e.acceptHookTermination(ctx, source.Key, entity.ExptStatus_Terminated, nil)
	return true, err
}

func (e *ExptMangerImpl) acceptHookTermination(ctx context.Context, key entity.HookRunKey, status entity.ExptStatus, display *string) (*entity.HookStoredRun, error) {
	acceptor, ok := e.finalization.Runs.(repo.IHookTerminationRepo)
	if !ok || !hookTerminationStatus(status) {
		return nil, entity.ErrHookFinalizationUnsupported
	}
	for attempt := 0; attempt < 5; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source, err := e.finalization.readSource(ctx, key)
		if err != nil {
			return nil, err
		}
		stored, err := e.finalization.loadNormalRun(ctx, source)
		if err != nil {
			return nil, err
		}
		intent := entity.HookTerminalIntent{Status: status}
		frozenDisplay := display
		if stored.State.Finalize != entity.HookFinalizeNone {
			if stored.State.Intent.Status != status || display != nil && (stored.DisplayMessage == nil || *display != *stored.DisplayMessage) {
				return nil, entity.ErrHookStoreConflict
			}
			intent = stored.State.Intent
			frozenDisplay = stored.DisplayMessage
		} else if frozenDisplay == nil {
			message, err := normalizeHookStatusMessage(gptr.Indirect(stored.DisplayMessage))
			if err != nil {
				return nil, err
			}
			frozenDisplay = &message
		}
		out, err := acceptor.AcceptTermination(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: stored.Version}, Intent: intent, DisplayMessage: frozenDisplay})
		if errors.Is(err, entity.ErrHookStoreConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if out.Run == nil || out.Run.State.Key != key || out.Run.State.Intent != intent || out.Run.State.Gate != entity.HookGateClosed || out.Run.State.Finalize == entity.HookFinalizeNone {
			return nil, entity.ErrHookStoreCorrupt
		}
		return out.Run, nil
	}
	return nil, entity.ErrHookStoreConflict
}

func waitHookCompletion(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (e *ExptMangerImpl) prepareHookTermination(ctx context.Context, source *entity.HookFinalizationSource, opt *entity.CompleteExptOption) error {
	stored, err := e.finalization.loadNormalRun(ctx, source)
	if err != nil {
		return err
	}
	// A standalone CompleteRun is not acceptance of the caller's later message/decision.
	if stored.State.Finalize == entity.HookFinalizeNone {
		return entity.ErrHookFinalizationUnsupported
	}
	if stored.State.Intent.Status != opt.Status {
		return entity.ErrHookStoreConflict
	}
	if opt.StatusMessage != "" {
		message, err := normalizeHookStatusMessage(opt.StatusMessage)
		if err != nil {
			return err
		}
		if stored.DisplayMessage == nil || message != *stored.DisplayMessage {
			return entity.ErrHookStoreConflict
		}
	}
	if err := waitHookCompletion(ctx, opt.CompleteInterval); err != nil {
		return err
	}
	if stored.State.Finalize == entity.HookFinalizeCommitted {
		return e.unlockFinalizedHookRun(ctx, source.Key)
	}
	if err := e.prepareActiveHookTermination(ctx, source, stored); err != nil {
		return err
	}
	_, err = e.finalization.readStats(ctx, source.Key)
	return err
}
