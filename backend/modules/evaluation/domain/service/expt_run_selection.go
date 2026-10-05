// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func readSingleSelectionPage(ctx context.Context, svc EvaluationSetItemService, spaceID, setID, versionID int64, size int32, token *string, ordered bool) ([]*entity.EvaluationSetItem, *int64, *string, error) {
	p := &entity.ListEvaluationSetItemsParam{SpaceID: spaceID, EvaluationSetID: setID, VersionID: resolveSetReadVersionID(setID, versionID), PageSize: &size, PageToken: token}
	if ordered {
		p.OrderBys = []*entity.OrderBy{{Field: gptr.Of("item_id"), IsAsc: gptr.Of(false)}}
	}
	items, total, _, next, err := svc.ListEvaluationSetItems(ctx, p)
	return items, total, next, err
}

func readExplicitSelectionItems(ctx context.Context, svc EvaluationSetItemService, spaceID, setID, versionID int64, ids []int64) ([]*entity.EvaluationSetItem, error) {
	return svc.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{SpaceID: spaceID, EvaluationSetID: setID, VersionID: resolveSetReadVersionID(setID, versionID), ItemIDs: append([]int64(nil), ids...)})
}

type setSelectionFilter struct {
	normal                     *entity.Filter
	tag                        *entity.TagFilter
	include, exclude           map[int64]struct{}
	includeCount, excludeCount int
}

func newSetSelectionFilter(f *entity.ExptItemFilter) (setSelectionFilter, error) {
	include, exclude, _, err := extractItemIDFilter(f)
	if err != nil {
		return setSelectionFilter{}, err
	}
	push := len(include) <= maxItemIDFilterInList && len(exclude) <= maxItemIDFilterInList
	out := setSelectionFilter{normal: extractNormalColumnFilter(f, push), tag: extractTagFilter(f), includeCount: len(include), excludeCount: len(exclude)}
	if out.normal != nil {
		for _, field := range out.normal.FilterFields {
			field.Values = append([]string(nil), field.Values...)
		}
	}
	if !push {
		out.include = make(map[int64]struct{}, len(include))
		out.exclude = make(map[int64]struct{}, len(exclude))
		for _, id := range include {
			out.include[id] = struct{}{}
		}
		for _, id := range exclude {
			out.exclude[id] = struct{}{}
		}
	}
	return out, nil
}

func readSetSelectionPage(ctx context.Context, svc EvaluationSetItemService, spaceID int64, sc *entity.EvalSetConfig, size int32, token *string, f setSelectionFilter) (items []*entity.EvaluationSetItem, total *int64, next *string, rawCount int, err error) {
	items, total, _, next, err = svc.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
		SpaceID: resolveLoadSpaceID(spaceID, sc.SourceSpaceID), EvaluationSetID: sc.EvalSetID, VersionID: resolveSetReadVersionID(sc.EvalSetID, sc.EvalSetVersionID), PageSize: &size, PageToken: token, Filter: f.normal, TagFilter: f.tag,
	})
	if err != nil {
		return nil, nil, nil, 0, err
	}
	rawCount = len(items)
	if len(f.include) > 0 || len(f.exclude) > 0 {
		kept := make([]*entity.EvaluationSetItem, 0, len(items))
		for _, it := range items {
			if it == nil {
				kept = append(kept, it)
				continue
			}
			if len(f.include) > 0 {
				if _, ok := f.include[it.ItemID]; !ok {
					continue
				}
			}
			if _, ok := f.exclude[it.ItemID]; ok {
				continue
			}
			kept = append(kept, it)
		}
		items = kept
	}
	return items, total, next, rawCount, nil
}
