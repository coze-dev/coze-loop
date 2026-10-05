// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type HookFrozenPlanLoaderDependencies struct {
	Plans    repo.IHookPlanRepo
	Items    EvaluationSetItemService
	Versions EvaluationSetVersionService
	Sets     IEvaluationSetService
	Resolver hook.FrozenContentResolver
}
type hookFrozenPlanLoader struct {
	deps HookFrozenPlanLoaderDependencies
}

func NewHookFrozenPlanLoader(d HookFrozenPlanLoaderDependencies) (hook.PlanPageLoader, error) {
	for _, dep := range []any{d.Plans, d.Items, d.Versions, d.Sets} {
		if missingManagerHookDependency(dep) {
			return nil, errors.New("missing frozen plan loader dependency")
		}
	}
	if d.Resolver != nil && missingManagerHookDependency(d.Resolver) {
		return nil, errors.New("invalid frozen content resolver")
	}
	return &hookFrozenPlanLoader{deps: d}, nil
}

type frozenGroupKey struct{ space, set, version int64 }
type frozenLoadGroup struct {
	key     frozenGroupKey
	indices []int
}
type frozenFieldSchema struct {
	name      string
	kind      entity.ContentType
	format    entity.FieldDisplayFormat
	schemaKey entity.SchemaKey
}

func (l *hookFrozenPlanLoader) LoadPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
	if ctx == nil || in.Validate() != nil {
		return nil, entity.ErrHookFrozenPlanInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	page, err := l.deps.Plans.ReadPlanPage(ctx, in)
	if err != nil || ctx.Err() != nil {
		return nil, frozenReadError(ctx, err, entity.ErrHookFrozenPlanInvalid)
	}
	if page == nil {
		return nil, entity.ErrHookFrozenPlanInvalid
	}
	if !page.Ready {
		return nil, entity.ErrHookFrozenPlanNotReady
	}
	return l.loadCandidates(ctx, in, page)
}

// LoadCandidates shares content verification with LoadPage; only the tail publisher persists these tuples.
func (l *hookFrozenPlanLoader) LoadCandidates(ctx context.Context, in entity.HookPlanReadInput, page *entity.HookPlanReadPage) (*entity.HookLoadedPlanPage, error) {
	if ctx == nil {
		return nil, entity.ErrHookFrozenPlanInvalid
	}
	return l.loadCandidates(context.WithValue(ctx, frozenCandidateIOKey{}, true), in, page)
}

type frozenCandidateIOKey struct{}

var errFrozenCandidateSourceRead = errors.New("frozen hook source read unavailable")

func (l *hookFrozenPlanLoader) loadCandidates(ctx context.Context, in entity.HookPlanReadInput, page *entity.HookPlanReadPage) (*entity.HookLoadedPlanPage, error) {
	if ctx == nil || in.Validate() != nil || page == nil || !page.Ready {
		return nil, entity.ErrHookFrozenPlanInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if page.RunVersion < 0 || page.Count < in.StartOrdinal || (entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Count: page.Count, Hash: page.Hash}).Validate() != nil {
		return nil, entity.ErrHookFrozenPlanInvalid
	}
	n := min(int64(in.Limit), page.Count-in.StartOrdinal)
	if int64(len(page.Items)) != n || page.NextOrdinal != in.StartOrdinal+n || page.HasMore != (page.NextOrdinal < page.Count) {
		return nil, entity.ErrHookFrozenPlanInvalid
	}
	tuples := append([]entity.HookPlanItem(nil), page.Items...)
	out := &entity.HookLoadedPlanPage{Count: page.Count, Hash: page.Hash, RunVersion: page.RunVersion, NextOrdinal: page.NextOrdinal, HasMore: page.HasMore, Items: make([]entity.HookLoadedPlanItem, len(tuples))}
	groups := make([]frozenLoadGroup, 0)
	byKey := map[frozenGroupKey]int{}
	pks := map[int64]bool{}
	members := map[frozenGroupKey]map[int64]bool{}
	for i, tuple := range tuples {
		if tuple.ID <= 0 || pks[tuple.ID] || tuple.SourceSpaceID <= 0 || tuple.EvalSetID <= 0 || tuple.EvalSetVersionID < 0 || tuple.ItemID <= 0 || tuple.ItemVersionID < 0 {
			return nil, entity.ErrHookFrozenPlanInvalid
		}
		pks[tuple.ID] = true
		version := tuple.EvalSetVersionID
		if isDraftEvalSet(tuple.EvalSetID, version) {
			version = 0
		}
		key := frozenGroupKey{tuple.SourceSpaceID, tuple.EvalSetID, version}
		if members[key] == nil {
			members[key] = map[int64]bool{}
		}
		if members[key][tuple.ItemID] {
			return nil, entity.ErrHookFrozenPlanInvalid
		}
		members[key][tuple.ItemID] = true
		group, exists := byKey[key]
		if !exists {
			group = len(groups)
			byKey[key] = group
			groups = append(groups, frozenLoadGroup{key: key})
		}
		groups[group].indices = append(groups[group].indices, i)
		out.Items[i] = entity.HookLoadedPlanItem{Ordinal: in.StartOrdinal + int64(i), Frozen: tuple}
	}
	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		schemaID, fields, err := l.schema(ctx, group.key)
		if err != nil {
			return nil, err
		}
		var zeroIDs []int64
		zeroIndices := map[int64]int{}
		for _, index := range group.indices {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			tuple := tuples[index]
			if tuple.ItemVersionID == 0 {
				zeroIDs = append(zeroIDs, tuple.ItemID)
				zeroIndices[tuple.ItemID] = index
				continue
			}
			v, err := l.deps.Items.GetEvaluationSetItemVersion(ctx, group.key.space, group.key.set, tuple.ItemID, gptr.Of(tuple.ItemVersionID), nil)
			if err != nil || ctx.Err() != nil {
				return nil, frozenReadError(ctx, err, entity.ErrHookFrozenItemUnavailable)
			}
			if v == nil || v.ItemID != tuple.ItemID || v.ItemVersionID != tuple.ItemVersionID {
				return nil, entity.ErrHookFrozenItemUnavailable
			}
			// The version DO has no dataset-item PK or source echo; these are request-bound references.
			item := &entity.EvaluationSetItem{SpaceID: group.key.space, EvaluationSetID: group.key.set, SchemaID: schemaID, ItemID: v.ItemID, ItemVersionID: gptr.Of(v.ItemVersionID), Turns: v.Turns, BaseInfo: v.BaseInfo}
			if v.Version != "" {
				item.ItemVersion = gptr.Of(v.Version)
			}
			out.Items[index].Item, err = l.prepareItem(ctx, tuple, item, fields)
			if err != nil {
				return nil, err
			}
		}
		if len(zeroIDs) > 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var version *int64
			if group.key.version > 0 {
				version = gptr.Of(group.key.version)
			}
			items, err := l.deps.Items.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{SpaceID: group.key.space, EvaluationSetID: group.key.set, ItemIDs: append([]int64(nil), zeroIDs...), VersionID: version})
			if err != nil || ctx.Err() != nil {
				return nil, frozenReadError(ctx, err, entity.ErrHookFrozenItemUnavailable)
			}
			if len(items) != len(zeroIDs) {
				return nil, entity.ErrHookFrozenItemUnavailable
			}
			seen := map[int64]bool{}
			for _, item := range items {
				if item == nil {
					return nil, entity.ErrHookFrozenItemUnavailable
				}
				index, exists := zeroIndices[item.ItemID]
				if !exists || seen[item.ItemID] || item.SpaceID != group.key.space || item.EvaluationSetID != group.key.set || item.ID < 0 || item.ItemVersionID != nil && *item.ItemVersionID < 0 {
					return nil, entity.ErrHookFrozenItemUnavailable
				}
				seen[item.ItemID] = true
				out.Items[index].Item, err = l.prepareItem(ctx, tuples[index], item, fields)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *hookFrozenPlanLoader) schema(ctx context.Context, key frozenGroupKey) (int64, map[string]frozenFieldSchema, error) {
	var schema *entity.EvaluationSetSchema
	if key.version > 0 {
		version, _, err := l.deps.Versions.GetEvaluationSetVersion(ctx, key.space, key.version, gptr.Of(true), nil)
		if err != nil || ctx.Err() != nil {
			return 0, nil, frozenReadError(ctx, err, entity.ErrHookFrozenSchemaInvalid)
		}
		if version == nil || version.ID != key.version || version.SpaceID != key.space || version.EvaluationSetID != key.set {
			return 0, nil, entity.ErrHookFrozenSchemaInvalid
		}
		schema = version.EvaluationSetSchema
	} else {
		set, err := l.deps.Sets.GetEvaluationSet(ctx, gptr.Of(key.space), key.set, gptr.Of(true), nil)
		if err != nil || ctx.Err() != nil {
			return 0, nil, frozenReadError(ctx, err, entity.ErrHookFrozenSchemaInvalid)
		}
		if set == nil || set.ID != key.set || set.SpaceID != key.space || set.EvaluationSetVersion == nil {
			return 0, nil, entity.ErrHookFrozenSchemaInvalid
		}
		schema = set.EvaluationSetVersion.EvaluationSetSchema
	}
	if schema == nil || schema.ID <= 0 || schema.SpaceID != key.space || schema.EvaluationSetID != key.set {
		return 0, nil, entity.ErrHookFrozenSchemaInvalid
	}
	fields := map[string]frozenFieldSchema{}
	keys, names := map[string]bool{}, map[string]bool{}
	for _, field := range schema.FieldSchemas {
		if field == nil || strings.TrimSpace(field.Key) == "" || !utf8.ValidString(field.Key) || keys[field.Key] {
			return 0, nil, entity.ErrHookFrozenSchemaInvalid
		}
		keys[field.Key] = true
		// DatasetSchema.AvailableFields accepts Available and the legacy empty status (DTO zero).
		if field.Status != 0 && field.Status != entity.FieldStatus_Available {
			continue
		}
		if strings.TrimSpace(field.Name) == "" || !utf8.ValidString(field.Name) || names[field.Name] {
			return 0, nil, entity.ErrHookFrozenSchemaInvalid
		}
		names[field.Name] = true
		fields[field.Key] = frozenFieldSchema{name: field.Name, kind: field.ContentType, format: field.DefaultDisplayFormat, schemaKey: gptr.Indirect(field.SchemaKey)}
	}
	return schema.ID, fields, nil
}

func (l *hookFrozenPlanLoader) prepareItem(ctx context.Context, tuple entity.HookPlanItem, item *entity.EvaluationSetItem, fields map[string]frozenFieldSchema) (*entity.EvaluationSetItem, error) {
	if len(item.Turns) == 0 {
		return nil, entity.ErrHookFrozenItemUnavailable
	}
	out := *item
	out.ItemVersionID = frozenPointer(item.ItemVersionID)
	out.ItemVersion = frozenPointer(item.ItemVersion)
	out.BaseInfo = cloneFrozenBase(item.BaseInfo)
	if item.Tags != nil {
		out.Tags = make([]*entity.ResourceTag, len(item.Tags))
		for i, tag := range item.Tags {
			out.Tags[i] = frozenPointer(tag)
		}
	}
	out.Turns = make([]*entity.Turn, len(item.Turns))
	seenTurns := map[int64]bool{}
	for i, turn := range item.Turns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if turn == nil || turn.ID < 0 || seenTurns[turn.ID] || turn.ItemID < 0 || turn.EvalSetID < 0 || turn.EvalSetID != 0 && turn.EvalSetID != tuple.EvalSetID {
			return nil, entity.ErrHookFrozenItemUnavailable
		}
		seenTurns[turn.ID] = true
		copy := *turn
		copy.FieldDataList = make([]*entity.FieldData, 0, len(turn.FieldDataList))
		seenFields := map[string]bool{}
		for _, field := range turn.FieldDataList {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if field == nil {
				return nil, entity.ErrHookFrozenItemUnavailable
			}
			schema, exists := fields[field.Key]
			if !exists {
				continue
			}
			if seenFields[field.Key] {
				return nil, entity.ErrHookFrozenItemUnavailable
			}
			seenFields[field.Key] = true
			content, err := cloneFrozenContent(field.Content, map[*entity.Content]bool{}, 0)
			if err != nil {
				return nil, err
			}
			defaultFrozenContent(content, schema)
			if !completeFrozenContent(content) {
				if l.deps.Resolver == nil {
					return nil, entity.ErrHookFrozenContentUnavailable
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				kind := content.GetContentType()
				if kind == "" {
					kind = schema.kind
				}
				resolved, err := l.deps.Resolver.Resolve(ctx, tuple, turn.ID, field.Key, content)
				if err != nil || ctx.Err() != nil {
					return nil, frozenReadError(ctx, err, entity.ErrHookFrozenContentUnavailable)
				}
				content, err = cloneFrozenContent(resolved, map[*entity.Content]bool{}, 0)
				if err != nil {
					return nil, err
				}
				defaultFrozenContent(content, schema)
				if !completeFrozenContent(content) || kind != "" && content.GetContentType() != kind {
					return nil, entity.ErrHookFrozenContentUnavailable
				}
				if schema.schemaKey == entity.SchemaKey_MessageList && !completeResolvedFrozenMessageList(content) {
					return nil, entity.ErrHookFrozenContentUnavailable
				}
			}
			fd := *field
			fd.Name = schema.name
			fd.Content = content
			copy.FieldDataList = append(copy.FieldDataList, &fd)
		}
		out.Turns[i] = &copy
	}
	return &out, nil
}

func frozenReadError(ctx context.Context, err, fallback error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, entity.ErrHookStoreMissing, entity.ErrHookStoreConflict, entity.ErrHookStoreCorrupt} {
		if errors.Is(err, cause) {
			return cause
		}
	}
	if candidate, _ := ctx.Value(frozenCandidateIOKey{}).(bool); candidate && err != nil {
		for _, cause := range []error{entity.ErrHookFrozenPlanInvalid, entity.ErrHookFrozenItemUnavailable, entity.ErrHookFrozenSchemaInvalid, entity.ErrHookFrozenContentUnavailable} {
			if errors.Is(err, cause) {
				return cause
			}
		}
		// Keep the failure class, never the upstream diagnostic or credentials.
		return errFrozenCandidateSourceRead
	}
	return fallback
}
