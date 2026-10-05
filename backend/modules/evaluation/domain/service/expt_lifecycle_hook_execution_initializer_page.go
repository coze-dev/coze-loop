// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"math"
	"reflect"
	"slices"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type executionInitializationPlan struct {
	count, version int64
	hash           string
	boundHash      string
}

func (p *executionInitializationPlan) observe(page *entity.HookExecutionInitializationPage, in entity.HookExecutionInitializationReadInput) error {
	if page == nil || page.RunVersion < p.version || !page.Initialized && page.RunVersion == math.MaxInt64 || page.Count < 0 || page.Count > math.MaxInt32 || page.NextOrdinal < 0 || page.NextOrdinal > page.Count {
		return entity.ErrHookStoreCorrupt
	}
	boundary := entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, ExecutionScope: in.ExecutionScope, PlanHash: page.Hash, ExpectedItemCount: page.Count}
	if boundary.Validate() != nil || p.hash != "" && (page.Hash != p.hash || page.Count != p.count || page.BoundSnapshotHash != p.boundHash) {
		return entity.ErrHookStoreCorrupt
	}
	if !page.Initialized || page.BoundSnapshotHash != "" {
		if in.StartOrdinal > page.Count {
			return entity.ErrHookStoreCorrupt
		}
		n := min(int64(in.Limit), page.Count-in.StartOrdinal)
		if int64(len(page.Items)) != n || page.NextOrdinal != in.StartOrdinal+n || page.HasMore != (page.NextOrdinal < page.Count) {
			return entity.ErrHookStoreCorrupt
		}
		ledgerIDs, itemIDs := map[int64]bool{}, map[int64]bool{}
		for i, item := range page.Items {
			f := item.Frozen
			if item.Ordinal != in.StartOrdinal+int64(i) || f.ID <= 0 || f.ItemID <= 0 || f.SourceSpaceID <= 0 || f.EvalSetID <= 0 || f.EvalSetVersionID < 0 || f.ItemVersionID < 0 || ledgerIDs[f.ID] || itemIDs[f.ItemID] {
				return entity.ErrHookStoreCorrupt
			}
			ledgerIDs[f.ID], itemIDs[f.ItemID] = true, true
			if m := item.Manifest; m != nil && (m.Validate() != nil || m.Key != in.Key || m.Ordinal != item.Ordinal || m.Frozen != f) {
				return entity.ErrHookStoreCorrupt
			}
			if m := item.Manifest; m != nil && ((m.ItemRef == nil) != (item.ItemRefConfigHash == "") || m.ItemRef != nil && m.ItemRef.ConfigHash != item.ItemRefConfigHash) {
				return entity.ErrHookStoreCorrupt
			}
		}
		if _, err := executionPageRecordIDs(page); err != nil {
			return err
		}
	}
	p.count, p.hash, p.version = page.Count, page.Hash, page.RunVersion
	p.boundHash = page.BoundSnapshotHash
	return nil
}

func executionPageMissing(page *entity.HookExecutionInitializationPage) bool {
	for _, item := range page.Items {
		if item.Manifest == nil {
			return true
		}
	}
	return false
}

func executionPageRecordIDs(page *entity.HookExecutionInitializationPage) (map[int64]bool, error) {
	ids := map[int64]bool{}
	for _, item := range page.Items {
		ids[item.Frozen.ID] = true
		ids[item.Frozen.ItemID] = true
	}
	for _, item := range page.Items {
		if item.Manifest == nil && item.Reuse == nil {
			continue
		}
		m := item.Manifest
		if m == nil {
			m = item.Reuse
		}
		if m.ItemRef != nil {
			if ids[m.ItemRef.ID] {
				return nil, entity.ErrHookStoreCorrupt
			}
			ids[m.ItemRef.ID] = true
		}
		for _, id := range []int64{m.ItemResultID, m.ItemRunLogID} {
			if id == 0 && item.Manifest == nil {
				continue
			}
			if id <= 0 || ids[id] {
				return nil, entity.ErrHookStoreCorrupt
			}
			ids[id] = true
		}
		for _, turn := range m.Turns {
			if turn.ResultID <= 0 || ids[turn.ResultID] {
				return nil, entity.ErrHookStoreCorrupt
			}
			ids[turn.ResultID] = true
		}
	}
	return ids, nil
}

func validateExecutionReceipt(read, receipt *entity.HookExecutionInitializationPage) error {
	if len(receipt.Items) != len(read.Items) {
		return entity.ErrHookStoreCorrupt
	}
	for i, old := range read.Items {
		got := receipt.Items[i]
		if got.Frozen != old.Frozen || got.Ordinal != old.Ordinal || got.ItemRefConfigHash != old.ItemRefConfigHash || receipt.BoundSnapshotHash != read.BoundSnapshotHash {
			return entity.ErrHookStoreCorrupt
		}
		if old.Manifest != nil {
			a, b := old.Manifest, got.Manifest
			if b == nil || a.Version != b.Version || a.Key != b.Key || a.Ordinal != b.Ordinal || a.Frozen != b.Frozen || a.ItemResultID != b.ItemResultID || a.ItemRunLogID != b.ItemRunLogID || !slices.Equal(a.Turns, b.Turns) || !reflect.DeepEqual(a.ItemRef, b.ItemRef) {
				return entity.ErrHookStoreCorrupt
			}
		}
	}
	return nil
}

func (s *hookFrozenExecutionInitializer) loadExecutionManifests(ctx context.Context, in entity.HookExecutionInitializationReadInput, page *entity.HookExecutionInitializationPage) ([]entity.HookExecutionManifest, error) {
	manifests := make([]entity.HookExecutionManifest, len(page.Items))
	var needed int64
	for i := 0; i < len(page.Items); {
		if m := page.Items[i].Manifest; m != nil {
			manifests[i] = m.Clone()
			i++
			continue
		}
		end := i + 1
		for end < len(page.Items) && page.Items[end].Manifest == nil {
			end++
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		loadInput := entity.HookPlanReadInput{Key: in.Key, ExecutionScope: in.ExecutionScope, StartOrdinal: in.StartOrdinal + int64(i), Limit: int32(end - i)}
		loaded, err := s.deps.Loader.LoadPage(ctx, loadInput)
		if err != nil || ctx.Err() != nil {
			return nil, executionInitializationError(ctx, err)
		}
		if loaded == nil || loaded.Count != page.Count || loaded.Hash != page.Hash || loaded.RunVersion < page.RunVersion || len(loaded.Items) != end-i || loaded.NextOrdinal != in.StartOrdinal+int64(end) || loaded.HasMore != (loaded.NextOrdinal < page.Count) {
			return nil, entity.ErrHookStoreCorrupt
		}
		for j, item := range loaded.Items {
			row := page.Items[i+j]
			manifest, err := executionManifestFromLoaded(in.Key, row, item)
			if err != nil {
				return nil, err
			}
			manifests[i+j] = manifest
			if manifest.ItemResultID == 0 {
				needed++
			}
			if manifest.ItemRunLogID == 0 {
				needed++
			}
			for _, tr := range manifest.Turns {
				if tr.ResultID == 0 {
					needed++
				}
			}
			if manifest.ItemRef != nil && manifest.ItemRef.ID == 0 {
				needed++
			}
		}
		i = end
	}
	if needed > int64(int(^uint(0)>>1)) {
		return nil, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := s.deps.IDs.GenMultiIDs(ctx, int(needed))
	if err != nil || ctx.Err() != nil {
		return nil, executionInitializationError(ctx, err)
	}
	if int64(len(ids)) != needed {
		return nil, entity.ErrHookStoreCorrupt
	}
	reserved, err := executionPageRecordIDs(page)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if id <= 0 || reserved[id] {
			return nil, entity.ErrHookStoreCorrupt
		}
		reserved[id] = true
	}
	next := 0
	for i := range manifests {
		if page.Items[i].Manifest != nil {
			continue
		}
		m := &manifests[i]
		if m.ItemResultID == 0 {
			m.ItemResultID = ids[next]
			next++
		}
		if m.ItemRunLogID == 0 {
			m.ItemRunLogID = ids[next]
			next++
		}
		if m.ItemRef != nil && m.ItemRef.ID == 0 {
			m.ItemRef.ID = ids[next]
			next++
		}
		for j := range m.Turns {
			if m.Turns[j].ResultID == 0 {
				m.Turns[j].ResultID = ids[next]
				next++
			}
		}
	}
	return manifests, nil
}

func executionManifestFromLoaded(key entity.HookRunKey, row entity.HookExecutionInitializationItem, loaded entity.HookLoadedPlanItem) (entity.HookExecutionManifest, error) {
	var empty entity.HookExecutionManifest
	f, item := row.Frozen, loaded.Item
	if loaded.Ordinal != row.Ordinal || loaded.Frozen != f || item == nil || item.ID < 0 || item.SpaceID != f.SourceSpaceID || item.EvaluationSetID != f.EvalSetID || item.ItemID != f.ItemID || len(item.Turns) == 0 || int64(len(item.Turns)) > math.MaxInt32 {
		return empty, entity.ErrHookStoreCorrupt
	}
	if item.ItemVersionID != nil && *item.ItemVersionID < 0 || f.ItemVersionID > 0 && (item.ItemVersionID == nil || *item.ItemVersionID != f.ItemVersionID) {
		return empty, entity.ErrHookStoreCorrupt
	}
	m := entity.HookExecutionManifest{Version: 1, Key: key, Ordinal: row.Ordinal, Frozen: f, Turns: make([]entity.HookExecutionTurnManifest, len(item.Turns)), TurnLogsInitialized: gptr.Of(false)}
	if row.ItemRefConfigHash != "" {
		m.ItemRef = &entity.HookExecutionItemRef{ConfigHash: row.ItemRefConfigHash}
	}
	seen := make(map[int64]bool, len(item.Turns))
	for i, turn := range item.Turns {
		if turn == nil || turn.ID < 0 || seen[turn.ID] {
			return empty, entity.ErrHookStoreCorrupt
		}
		seen[turn.ID] = true
		m.Turns[i] = entity.HookExecutionTurnManifest{TurnID: turn.ID, TurnIdx: int32(i)}
	}
	if old := row.Reuse; old != nil {
		if old.Key != key || old.Frozen != f || old.Ordinal != row.Ordinal || len(old.Turns) != len(m.Turns) {
			return empty, entity.ErrHookStoreCorrupt
		}
		for i, t := range old.Turns {
			if t.TurnID != m.Turns[i].TurnID || t.TurnIdx != m.Turns[i].TurnIdx {
				return empty, entity.ErrHookStoreCorrupt
			}
		}
		return old.Clone(), nil
	}
	return m, nil
}
