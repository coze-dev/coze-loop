// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type itemHookTurnIdentityKey struct{}
type itemHookTurnIdentity struct {
	key                 entity.HookRunKey
	item, turn, version int64
}

func managedItemHookVersion(ctx context.Context, eiec *entity.ExptItemEvalCtx) (int64, error) {
	b, ok := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !ok {
		return 0, nil
	}
	if eiec == nil || eiec.Event == nil || b.key != itemHookKey(eiec.Event) || b.itemID != eiec.Event.EvalSetItemID || b.itemVersion < 0 {
		return 0, itemHookControlError{wait: true}
	}
	if row := eiec.GetExistItemResultLog(); row != nil && (row.SpaceID != b.key.WorkspaceID || row.ExptID != b.key.ExperimentID || row.ExptRunID != b.key.RunID || row.ItemID != b.itemID || row.ItemVersionID != b.itemVersion) {
		return 0, itemHookControlError{wait: true}
	}
	return b.itemVersion, nil
}

func bindItemHookTurnIdentity(ctx context.Context, etec *entity.ExptTurnEvalCtx) (context.Context, error) {
	if ctx.Value(itemHookProgressContextKey{}) == nil {
		return ctx, nil
	}
	version, err := managedItemHookVersion(ctx, etec.ExptItemEvalCtx)
	if err != nil {
		return ctx, err
	}
	b, err := itemHookProgressFor(ctx, etec)
	if err != nil {
		return ctx, err
	}
	k := entity.HookTurnProgressIdentity(etec.GetExistTurnResultRunLog(etec.Turn.ID))
	if k.Validate() != nil || k.HookRunKey != b.key || k.ItemID != b.itemID || k.TurnID != etec.Turn.ID || k.ItemVersionID != version {
		return ctx, itemHookControlError{wait: true}
	}
	return context.WithValue(ctx, itemHookTurnIdentityKey{}, itemHookTurnIdentity{key: b.key, item: b.itemID, turn: k.TurnID, version: version}), nil
}

// This identity is minted from frozen runlogs after admission, never business Ext.
func hookRecordItemVersion(ctx context.Context, space, expt, run, item, turn int64) (int64, error) {
	identity, ok := ctx.Value(itemHookTurnIdentityKey{}).(itemHookTurnIdentity)
	b, managed := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !managed && !ok {
		return 0, nil
	}
	if !managed || !ok || identity.key != b.key || identity.item != b.itemID || identity.version != b.itemVersion || identity.key != (entity.HookRunKey{WorkspaceID: space, ExperimentID: expt, RunID: run}) || identity.item != item || identity.turn != turn {
		return 0, itemHookControlError{wait: true}
	}
	return identity.version, nil
}
