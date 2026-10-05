// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

type hookPlanSelector struct {
	items   EvaluationSetItemService
	refs    repo.IExptItemRefRepo
	turns   repo.IExptTurnResultRepo
	results repo.IExptItemResultRepo
}

func NewHookPlanSelector(items EvaluationSetItemService, refs repo.IExptItemRefRepo, turns repo.IExptTurnResultRepo, results repo.IExptItemResultRepo) hook.PlanSelector {
	return &hookPlanSelector{items: items, refs: refs, turns: turns, results: results}
}

func (s *hookPlanSelector) SelectPage(ctx context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
	c, err := selectionCursor(in)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if err = ctx.Err(); err != nil {
		return entity.HookSelectionPage{}, err
	}
	if c.Done {
		return selectionPage(nil, c, true)
	}
	switch in.Mode {
	case entity.EvaluationModeAppend:
		if in.Cursor != "" {
			return entity.HookSelectionPage{}, errSelectionCursor
		}
		return selectionPage(nil, c, true)
	case entity.EvaluationModeRetryItems:
		return s.selectIDs(ctx, in, c, true)
	case entity.EvaluationModeFailRetry:
		return s.selectFailed(ctx, in, c)
	case entity.EvaluationModeRetryAll:
		if in.Experiment.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
			return s.selectRefs(ctx, in, c)
		}
	case entity.EvaluationModeTrialRun:
		if in.Experiment.TrialRunItemCount > 0 {
			if in.HasExplicitItemIDs && len(in.ItemIDs) > 0 {
				return s.selectIDs(ctx, in, c, false)
			}
			if !in.HasExplicitItemIDs {
				return s.selectSingle(ctx, in, c, true)
			}
		}
	}
	if in.Experiment.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		return s.selectSets(ctx, in, c)
	}
	return s.selectSingle(ctx, in, c, false)
}

var (
	errSelectionCursor = errors.New("invalid hook selection input or cursor")
	errSelectionSource = errors.New("invalid hook selection source page")
)

const (
	selectionPageSize       = 100
	selectionMaxSourcePages = 10000
)

// Read retries are allowed. The preparation repository owns cursor-commit CAS.
type hookSelectionCursor struct {
	Version     int    `json:"v"`
	Fingerprint string `json:"f"`
	Set         int    `json:"s,omitempty"`
	Token       string `json:"t,omitempty"`
	Raw         int64  `json:"r,omitempty"`
	Selected    int64  `json:"n,omitempty"`
	Total       int64  `json:"total,omitempty"` // Decode older v2 cursors; never a source-exhaustion signal.
	Offset      int    `json:"o,omitempty"`
	PK          int64  `json:"p,omitempty"`
	Pages       int    `json:"pages,omitempty"`
	CycleAnchor string `json:"anchor,omitempty"`
	CyclePower  int    `json:"power,omitempty"`
	CycleSpan   int    `json:"span,omitempty"`
	Done        bool   `json:"d,omitempty"`
}

func selectionCursor(in entity.HookSelectionInput) (hookSelectionCursor, error) {
	e := in.Experiment
	if in.Key.WorkspaceID <= 0 || in.Key.ExperimentID <= 0 || in.Key.RunID <= 0 || e == nil || e.ID != in.Key.ExperimentID || e.SpaceID != in.Key.WorkspaceID || in.Mode < 1 || in.Mode > 6 || e.EvalSetSourceType < 0 || e.EvalSetSourceType > 2 {
		return hookSelectionCursor{}, errSelectionCursor
	}
	for _, id := range in.ItemIDs {
		if id <= 0 {
			return hookSelectionCursor{}, errSelectionCursor
		}
	}
	// Only read-affecting fields enter the digest; configuration/identity never enter the cursor.
	type setKey struct {
		ID, Version, Space int64
		Filter             *entity.ExptItemFilter
	}
	key := struct {
		Run      entity.HookRunKey
		Mode     entity.ExptRunMode
		Source   entity.ExptEvalSetSourceType
		Single   setKey
		Sets     []setKey
		Count    int64
		Explicit bool
		IDs      []int64
	}{Run: in.Key, Mode: in.Mode, Source: e.EvalSetSourceType, Count: e.TrialRunItemCount, Explicit: in.HasExplicitItemIDs, IDs: in.ItemIDs}
	if e.EvalSet != nil {
		key.Single.ID = e.EvalSet.ID
		key.Single.Space = e.EvalSetSpaceID
		if e.EvalSet.EvaluationSetVersion != nil {
			key.Single.Version = e.EvalSet.EvaluationSetVersion.ID
		}
	}
	if e.EvalConf != nil {
		for _, sc := range e.EvalConf.EvalSetConfigs {
			if sc == nil {
				key.Sets = append(key.Sets, setKey{})
				continue
			}
			key.Sets = append(key.Sets, setKey{sc.EvalSetID, sc.EvalSetVersionID, sc.SourceSpaceID, sc.ItemFilter})
		}
	}
	data, err := json.Marshal(key)
	if err != nil {
		return hookSelectionCursor{}, errSelectionCursor
	}
	sum := sha256.Sum256(data)
	fingerprint := hex.EncodeToString(sum[:])
	c := hookSelectionCursor{Version: 2, Fingerprint: fingerprint}
	if in.Cursor == "" {
		return c, nil
	}
	if len(in.Cursor) > 65535 || !utf8.ValidString(in.Cursor) {
		return c, errSelectionCursor
	}
	decoder := json.NewDecoder(strings.NewReader(in.Cursor))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&c); err != nil {
		return c, errSelectionCursor
	}
	canonical, _ := json.Marshal(c)
	if string(canonical) != in.Cursor || c.Version != 2 || c.Fingerprint != fingerprint || c.Set < 0 || c.Raw < 0 || c.Selected < 0 || c.Offset < 0 || c.Offset > len(in.ItemIDs) || c.PK < 0 {
		return c, errSelectionCursor
	}
	if c.Token != "" {
		if c.Pages < 1 || c.Pages >= selectionMaxSourcePages || c.CyclePower < 1 || c.CyclePower > c.Pages || c.CyclePower&(c.CyclePower-1) != 0 || c.CycleSpan < 0 || c.CycleSpan >= c.CyclePower || c.CycleSpan != c.Pages-c.CyclePower {
			return c, errSelectionCursor
		}
		decoded, decodeErr := hex.DecodeString(c.CycleAnchor)
		if decodeErr != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != c.CycleAnchor {
			return c, errSelectionCursor
		}
		if c.CycleSpan == 0 {
			hash := sha256.Sum256([]byte(c.Token))
			if c.CycleAnchor != hex.EncodeToString(hash[:]) {
				return c, errSelectionCursor
			}
		}
	} else if c.Raw != 0 || c.Pages != 0 || c.CyclePower != 0 || c.CycleSpan != 0 || c.CycleAnchor != "" {
		return c, errSelectionCursor
	}
	if c.Done && (c.Set != 0 || c.Token != "" || c.Raw != 0 || c.Selected != 0 || c.Total != 0 || c.Offset != 0 || c.PK != 0) {
		return c, errSelectionCursor
	}
	return c, nil
}

func selectionPage(items []entity.HookPlanItem, c hookSelectionCursor, done bool) (entity.HookSelectionPage, error) {
	if !done && c.Pages >= selectionMaxSourcePages {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	page := entity.HookSelectionPage{Items: items, Done: done}
	if len(items) > selectionPageSize {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	for _, item := range items {
		if item.ID != 0 || item.ItemID <= 0 || item.EvalSetID <= 0 || item.SourceSpaceID <= 0 || item.ItemVersionID < 0 || item.EvalSetVersionID < 0 {
			return entity.HookSelectionPage{}, errSelectionSource
		}
	}
	if done {
		c = hookSelectionCursor{Version: c.Version, Fingerprint: c.Fingerprint, Done: true}
	}
	b, err := json.Marshal(c)
	if err != nil || len(b) > 65535 {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	page.NextCursor = string(b)
	return page, nil
}

func advanceSelectionToken(c *hookSelectionCursor, next *string) error {
	if c.Pages >= selectionMaxSourcePages {
		return errSelectionSource
	}
	c.Pages++
	token := gptr.Indirect(next)
	if token == "" {
		return nil
	}
	if !utf8.ValidString(token) || token == c.Token {
		return errSelectionSource
	}
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	// Brent's checkpoint moves at powers of two; persisting it preserves cycle
	// detection across worker restarts without retaining the token history.
	if c.CyclePower == 0 {
		c.CycleAnchor, c.CyclePower = hash, 1
	} else {
		c.CycleSpan++
		if c.CycleAnchor == hash {
			return errSelectionSource
		}
		if c.CycleSpan == c.CyclePower {
			c.CycleAnchor = hash
			c.CyclePower *= 2
			c.CycleSpan = 0
		}
	}
	c.Token = token
	return nil
}

func singleSelectionSource(in entity.HookSelectionInput) (space, set, version int64, err error) {
	e := in.Experiment
	if e.EvalSet == nil || e.EvalSet.ID <= 0 || e.EvalSet.EvaluationSetVersion == nil || e.EvalSet.EvaluationSetVersion.ID < 0 || e.EvalSetSpaceID < 0 {
		return 0, 0, 0, errSelectionCursor
	}
	return resolveLoadSpaceID(in.Key.WorkspaceID, e.EvalSetSpaceID), e.EvalSet.ID, e.EvalSet.EvaluationSetVersion.ID, nil
}

func selectionCandidates(items []*entity.EvaluationSetItem, space, set, version int64) ([]entity.HookPlanItem, error) {
	out := make([]entity.HookPlanItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			return nil, errSelectionSource
		}
		out = append(out, entity.HookPlanItem{SourceSpaceID: space, EvalSetID: set, EvalSetVersionID: resolveSetRefVersionID(set, version), ItemID: item.ItemID, ItemVersionID: gptr.Indirect(item.ItemVersionID)})
	}
	return out, nil
}

func (s *hookPlanSelector) selectSingle(ctx context.Context, in entity.HookSelectionInput, c hookSelectionCursor, trial bool) (entity.HookSelectionPage, error) {
	if c.Set != 0 || c.Offset != 0 || c.PK != 0 || (in.Cursor != "" && c.Token == "") {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	space, set, version, err := singleSelectionSource(in)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if s.items == nil {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	size := int32(selectionPageSize)
	if trial && in.Experiment.TrialRunItemCount < int64(size) {
		size = int32(in.Experiment.TrialRunItemCount)
	}
	var token *string
	if c.Token != "" {
		token = gptr.Of(c.Token)
	}
	items, _, next, err := readSingleSelectionPage(ctx, s.items, space, set, version, size, token, trial)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if len(items) > int(size) {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	if err = advanceSelectionToken(&c, next); err != nil {
		return entity.HookSelectionPage{}, err
	}
	raw := len(items)
	c.Raw += int64(raw)
	if trial {
		remain := in.Experiment.TrialRunItemCount - c.Selected
		if remain <= 0 {
			return entity.HookSelectionPage{}, errSelectionCursor
		}
		if int64(len(items)) > remain {
			items = items[:int(remain)]
		}
	}
	c.Selected += int64(len(items))
	// Total is cached independently of this source page. A valid next token
	// must be consumed even when Total or the current page's item count is zero.
	done := gptr.Indirect(next) == ""
	if trial {
		done = done || c.Selected >= in.Experiment.TrialRunItemCount
	}
	candidates, err := selectionCandidates(items, space, set, version)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	return selectionPage(candidates, c, done)
}

func (s *hookPlanSelector) selectSets(ctx context.Context, in entity.HookSelectionInput, c hookSelectionCursor) (entity.HookSelectionPage, error) {
	if c.Offset != 0 || c.PK != 0 {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	e := in.Experiment
	if e.EvalConf == nil || len(e.EvalConf.EvalSetConfigs) == 0 {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	sets := e.EvalConf.EvalSetConfigs
	if c.Set >= len(sets) {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	for c.Set < len(sets) && sets[c.Set] == nil {
		c.Set++
	}
	if c.Set == len(sets) {
		return selectionPage(nil, c, true)
	}
	sc := sets[c.Set]
	if sc.EvalSetID <= 0 || sc.EvalSetVersionID < 0 || sc.SourceSpaceID < 0 || s.items == nil {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	filter, err := newSetSelectionFilter(sc.ItemFilter)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	var token *string
	if c.Token != "" {
		token = gptr.Of(c.Token)
	}
	items, _, next, raw, err := readSetSelectionPage(ctx, s.items, in.Key.WorkspaceID, sc, selectionPageSize, token, filter)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if raw > selectionPageSize {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	if err = advanceSelectionToken(&c, next); err != nil {
		return entity.HookSelectionPage{}, err
	}
	c.Raw += int64(raw)
	c.Selected += int64(len(items))
	candidates, err := selectionCandidates(items, resolveLoadSpaceID(in.Key.WorkspaceID, sc.SourceSpaceID), sc.EvalSetID, sc.EvalSetVersionID)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if gptr.Indirect(next) == "" {
		c.Set++
		c.Token = ""
		c.Raw = 0
		c.Pages = 0
		c.CycleAnchor = ""
		c.CyclePower = 0
		c.CycleSpan = 0
		for c.Set < len(sets) && sets[c.Set] == nil {
			c.Set++
		}
	}
	return selectionPage(candidates, c, c.Set == len(sets))
}

func (s *hookPlanSelector) selectIDs(ctx context.Context, in entity.HookSelectionInput, c hookSelectionCursor, retry bool) (entity.HookSelectionPage, error) {
	if c.Set != 0 || c.Token != "" || c.PK != 0 || c.Raw != 0 || c.Selected != 0 || (in.Cursor != "" && (c.Offset == 0 || c.Offset == len(in.ItemIDs))) {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	end := c.Offset + selectionPageSize
	if end > len(in.ItemIDs) {
		end = len(in.ItemIDs)
	}
	ids := append([]int64(nil), in.ItemIDs[c.Offset:end]...)
	c.Offset = end
	if len(ids) == 0 {
		return selectionPage(nil, c, true)
	}
	if retry && in.Experiment.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		refs, err := s.readMatchingRefs(ctx, in, ids)
		if err != nil {
			return entity.HookSelectionPage{}, err
		}
		items, err := s.hydrateRefs(ctx, in, refs)
		if err != nil {
			return entity.HookSelectionPage{}, err
		}
		return selectionPage(items, c, end == len(in.ItemIDs))
	}
	space, set, version, err := singleSelectionSource(in)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if s.items == nil {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	items, err := readExplicitSelectionItems(ctx, s.items, space, set, version, ids)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	candidates, err := selectionCandidates(items, space, set, version)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if retry {
		if s.results == nil {
			return entity.HookSelectionPage{}, errSelectionSource
		}
		results, err := s.results.MGetItemResults(ctx, in.Key.ExperimentID, ids, in.Key.WorkspaceID)
		if err != nil {
			return entity.HookSelectionPage{}, err
		}
		versions := make(map[int64]int64, len(results))
		for _, r := range results {
			if r != nil {
				versions[r.ItemID] = r.ItemVersionID
			}
		}
		for i := range candidates {
			candidates[i].ItemVersionID = versions[candidates[i].ItemID]
		}
	}
	allowed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	for _, item := range candidates {
		if !allowed[item.ItemID] {
			return entity.HookSelectionPage{}, errSelectionSource
		}
	}
	return selectionPage(candidates, c, end == len(in.ItemIDs))
}

func cloneSelectionRefs(refs []*entity.ExptItemRef, e *entity.Experiment) ([]*entity.ExptItemRef, error) {
	cloned := make([]*entity.ExptItemRef, 0, len(refs))
	seen := make(map[int64]bool, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != e.SpaceID || ref.ExptID != e.ID || ref.ItemID <= 0 || ref.EvalSetID <= 0 || ref.ItemVersionID < 0 || ref.EvalSetVersionID < 0 || seen[ref.ItemID] {
			return nil, errSelectionSource
		}
		value := *ref
		cloned = append(cloned, &value)
		seen[ref.ItemID] = true
	}
	backfillRefEvalSetSourceSpace(cloned, e)
	return cloned, nil
}

func (s *hookPlanSelector) readMatchingRefs(ctx context.Context, in entity.HookSelectionInput, ids []int64) ([]*entity.ExptItemRef, error) {
	if s.refs == nil {
		return nil, errSelectionSource
	}
	refs, err := s.refs.MGetByExptIDAndItemIDs(ctx, in.Key.WorkspaceID, in.Key.ExperimentID, append([]int64(nil), ids...))
	if err != nil {
		return nil, err
	}
	refs, err = cloneSelectionRefs(refs, in.Experiment)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*entity.ExptItemRef, len(refs))
	for _, ref := range refs {
		byID[ref.ItemID] = ref
	}
	if len(refs) != len(ids) {
		return nil, errSelectionSource
	}
	ordered := make([]*entity.ExptItemRef, 0, len(ids))
	for _, id := range ids {
		ref := byID[id]
		if ref == nil {
			return nil, errSelectionSource
		}
		ordered = append(ordered, ref)
		delete(byID, id)
	}
	return ordered, nil
}

func refSelectionCandidate(ref *entity.ExptItemRef, space int64) entity.HookPlanItem {
	return entity.HookPlanItem{SourceSpaceID: resolveLoadSpaceID(space, ref.EvalSetSourceSpaceID), EvalSetID: ref.EvalSetID, EvalSetVersionID: resolveSetRefVersionID(ref.EvalSetID, ref.EvalSetVersionID), ItemID: ref.ItemID, ItemVersionID: ref.ItemVersionID}
}

func (s *hookPlanSelector) hydrateRefs(ctx context.Context, in entity.HookSelectionInput, refs []*entity.ExptItemRef) ([]entity.HookPlanItem, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	if s.items == nil {
		return nil, errSelectionSource
	}
	items, _, err := fetchEvaluationSetItemsByRefs(ctx, retryItemResetDeps{evaluationSetItemService: s.items}, in.Key.WorkspaceID, refs)
	if err != nil {
		return nil, err
	}
	available := make(map[int64]*entity.EvaluationSetItem, len(items))
	for _, item := range items {
		if item == nil || available[item.ItemID] != nil {
			return nil, errSelectionSource
		}
		available[item.ItemID] = item
	}
	out := make([]entity.HookPlanItem, 0, len(refs))
	for _, ref := range refs {
		item := available[ref.ItemID]
		if item == nil {
			return nil, fmt.Errorf("%w: missing referenced item", errSelectionSource)
		}
		candidate := refSelectionCandidate(ref, in.Key.WorkspaceID)
		if version := gptr.Indirect(item.ItemVersionID); version != 0 {
			candidate.ItemVersionID = version
		}
		out = append(out, candidate)
		delete(available, ref.ItemID)
	}
	if len(available) > 0 {
		return nil, errSelectionSource
	}
	return out, nil
}

func (s *hookPlanSelector) selectRefs(ctx context.Context, in entity.HookSelectionInput, c hookSelectionCursor) (entity.HookSelectionPage, error) {
	if c.Set != 0 || c.Offset != 0 || c.Token != "" || c.Raw != 0 || c.Selected != 0 || (in.Cursor != "" && c.PK == 0) {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	if s.refs == nil {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	refs, next, err := s.refs.ListByExptID(ctx, in.Key.WorkspaceID, in.Key.ExperimentID, c.PK, selectionPageSize)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if len(refs) > selectionPageSize || next < 0 || (next != 0 && (len(refs) == 0 || next <= c.PK)) {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	refs, err = cloneSelectionRefs(refs, in.Experiment)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	last := c.PK
	for _, ref := range refs {
		if ref.ID <= last {
			return entity.HookSelectionPage{}, errSelectionSource
		}
		last = ref.ID
	}
	if next != 0 && next != last {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	items, err := s.hydrateRefs(ctx, in, refs)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	c.PK = next
	return selectionPage(items, c, next == 0)
}

func (s *hookPlanSelector) selectFailed(ctx context.Context, in entity.HookSelectionInput, c hookSelectionCursor) (entity.HookSelectionPage, error) {
	if c.Set != 0 || c.Offset != 0 || c.Token != "" || c.Raw != 0 || c.Selected != 0 || (in.Cursor != "" && c.PK == 0) {
		return entity.HookSelectionPage{}, errSelectionCursor
	}
	if s.turns == nil {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	status := []int32{int32(entity.TurnRunState_Terminal), int32(entity.TurnRunState_Queueing), int32(entity.TurnRunState_Fail), int32(entity.TurnRunState_Processing)}
	turns, next, err := s.turns.ScanTurnResults(contexts.WithCtxWriteDB(ctx), in.Key.ExperimentID, status, c.PK, 50, in.Key.WorkspaceID)
	if err != nil {
		return entity.HookSelectionPage{}, err
	}
	if len(turns) > 50 || next < 0 || (len(turns) > 0 && next <= c.PK) || (len(turns) == 0 && next != 0) {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	versions := make(map[int64]int64, len(turns))
	ids := make([]int64, 0, len(turns))
	last := c.PK
	for _, tr := range turns {
		if tr == nil || tr.ID <= last || tr.ItemID <= 0 || tr.ItemVersionID < 0 {
			return entity.HookSelectionPage{}, errSelectionSource
		}
		last = tr.ID
		if v, ok := versions[tr.ItemID]; ok {
			if v != tr.ItemVersionID {
				return entity.HookSelectionPage{}, errSelectionSource
			}
		} else {
			ids = append(ids, tr.ItemID)
		}
		versions[tr.ItemID] = tr.ItemVersionID
	}
	if len(turns) > 0 && next != last {
		return entity.HookSelectionPage{}, errSelectionSource
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	c.PK = next
	if len(ids) == 0 {
		return selectionPage(nil, c, true)
	}
	out := make([]entity.HookPlanItem, 0, len(ids))
	if in.Experiment.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		refs, err := s.readMatchingRefs(ctx, in, ids)
		if err != nil {
			return entity.HookSelectionPage{}, err
		}
		for _, ref := range refs {
			item := refSelectionCandidate(ref, in.Key.WorkspaceID)
			item.ItemVersionID = versions[item.ItemID]
			out = append(out, item)
		}
	} else {
		space, set, version, err := singleSelectionSource(in)
		if err != nil {
			return entity.HookSelectionPage{}, err
		}
		for _, id := range ids {
			out = append(out, entity.HookPlanItem{SourceSpaceID: space, EvalSetID: set, EvalSetVersionID: resolveSetRefVersionID(set, version), ItemID: id, ItemVersionID: versions[id]})
		}
	}
	return selectionPage(out, c, false)
}
