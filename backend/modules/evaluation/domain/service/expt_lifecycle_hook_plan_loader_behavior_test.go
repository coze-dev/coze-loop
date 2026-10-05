// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookFrozenLoaderRejectsPlanShapesBeforeSources(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*loaderFixture)
	}{
		{"nil", func(f *loaderFixture) { f.plans.page = nil }}, {"not_ready", func(f *loaderFixture) { f.plans.page.Ready = false }},
		{"bad_limit", func(f *loaderFixture) { f.input.Limit = 101 }}, {"bad_scope", func(f *loaderFixture) { f.input.ExecutionScope = "" }},
		{"bad_key", func(f *loaderFixture) { f.input.Key.RunID = 0 }}, {"hash", func(f *loaderFixture) { f.plans.page.Hash = "" }},
		{"false_empty", func(f *loaderFixture) { f.plans.page.Items = nil }}, {"count", func(f *loaderFixture) { f.plans.page.Count = -1 }},
		{"ordinal", func(f *loaderFixture) { f.plans.page.NextOrdinal = 0 }}, {"tail", func(f *loaderFixture) { f.plans.page.HasMore = true }},
		{"run_version", func(f *loaderFixture) { f.plans.page.RunVersion = -1 }}, {"zero_source", func(f *loaderFixture) { f.plans.page.Items[0].SourceSpaceID = 0 }},
		{"negative_item_version", func(f *loaderFixture) { f.plans.page.Items[0].ItemVersionID = -1 }}, {"duplicate_plan_pk", func(f *loaderFixture) {
			f.plans.page.Items = append(f.plans.page.Items, f.plans.page.Items[0])
			f.plans.page.Count = 2
			f.plans.page.NextOrdinal = 2
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLoaderFixture()
			tc.change(f)
			out, err := f.load(t)
			require.Error(t, err)
			require.Nil(t, out)
			require.Zero(t, f.items.batchCalls+f.items.versionCalls+f.versions.calls+f.sets.calls)
		})
	}
	f := newLoaderFixture()
	f.plans.page.Items = nil
	f.plans.page.Count = 0
	f.plans.page.NextOrdinal = 0
	out, err := f.load(t)
	require.NoError(t, err)
	require.Empty(t, out.Items)
	require.Equal(t, f.plans.page.Hash, out.Hash)
	require.Zero(t, f.items.batchCalls+f.versions.calls)
}

func exactLoaderFixture() *loaderFixture {
	f := newLoaderFixture()
	f.plans.page.Items[0].ItemVersionID = 4
	f.items.version = &entity.EvaluationSetItemVersion{ItemID: 30, ItemVersionID: 4, Version: "v4", Turns: []*entity.Turn{{ID: 7, FieldDataList: []*entity.FieldData{{Key: "key-q", Content: &entity.Content{Text: gptr.Of("turn one")}}}}, {ID: 0, FieldDataList: []*entity.FieldData{{Key: "key-q", Content: &entity.Content{Text: gptr.Of("")}}}}}}
	return f
}

func TestHookFrozenLoaderExactVersionPreservesTurnsWithoutFakePK(t *testing.T) {
	f := exactLoaderFixture()
	page, err := f.load(t)
	require.NoError(t, err)
	item := page.Items[0].Item
	require.Zero(t, item.ID)
	require.Equal(t, int64(30), item.ItemID)
	require.Equal(t, int64(4), *item.ItemVersionID)
	require.Equal(t, int64(7), item.Turns[0].ID)
	require.Zero(t, item.Turns[1].ID)
	require.Zero(t, item.Turns[0].ItemID)
	require.Equal(t, "", *item.Turns[1].FieldDataList[0].Content.Text)
	require.Zero(t, f.items.batchCalls)
	*item.Turns[0].FieldDataList[0].Content.Text = "changed"
	require.Equal(t, "turn one", *f.items.version.Turns[0].FieldDataList[0].Content.Text)
	for _, kind := range []string{"nil_stub", "wrong_item", "wrong_version", "missing_turns", "nil_turn", "duplicate_turn", "negative_turn", "wrong_turn_set"} {
		t.Run(kind, func(t *testing.T) {
			f := exactLoaderFixture()
			switch kind {
			case "nil_stub":
				f.items.version = nil
			case "wrong_item":
				f.items.version.ItemID++
			case "wrong_version":
				f.items.version.ItemVersionID++
			case "missing_turns":
				f.items.version.Turns = nil
			case "nil_turn":
				f.items.version.Turns[0] = nil
			case "duplicate_turn":
				f.items.version.Turns[1].ID = 7
			case "negative_turn":
				f.items.version.Turns[0].ID = -1
			case "wrong_turn_set":
				f.items.version.Turns[0].EvalSetID = 999
			}
			out, err := f.load(t)
			require.ErrorIs(t, err, entity.ErrHookFrozenItemUnavailable)
			require.Nil(t, out)
			require.Zero(t, f.items.batchCalls)
		})
	}
}

func TestHookFrozenLoaderBatchCompletenessAndScope(t *testing.T) {
	for _, kind := range []string{"missing", "extra", "wrong_item", "wrong_space", "missing_space", "wrong_set", "nil"} {
		t.Run(kind, func(t *testing.T) {
			f := newLoaderFixture()
			switch kind {
			case "missing":
				f.items.batch = nil
			case "extra":
				f.items.batch = append(f.items.batch, f.items.batch[0])
			case "wrong_item":
				f.items.batch[0].ItemID++
			case "wrong_space":
				f.items.batch[0].SpaceID++
			case "missing_space":
				f.items.batch[0].SpaceID = 0
			case "wrong_set":
				f.items.batch[0].EvaluationSetID++
			case "nil":
				f.items.batch[0] = nil
			}
			out, err := f.load(t)
			require.ErrorIs(t, err, entity.ErrHookFrozenItemUnavailable)
			require.Nil(t, out)
		})
	}
	f := newLoaderFixture()
	second := f.plans.page.Items[0]
	second.ID = 901
	second.ItemID = 31
	f.plans.page.Items = append(f.plans.page.Items, second)
	f.plans.page.Count = 2
	f.plans.page.NextOrdinal = 2
	f.items.batch = append(f.items.batch, f.items.batch[0])
	_, err := f.load(t)
	require.ErrorIs(t, err, entity.ErrHookFrozenItemUnavailable)
	f = newLoaderFixture()
	f.items.batch[0].ItemVersionID = gptr.Of(int64(99))
	out, err := f.load(t)
	require.NoError(t, err)
	require.Zero(t, out.Items[0].Frozen.ItemVersionID)
	require.Equal(t, int64(99), *out.Items[0].Item.ItemVersionID)
}

func TestHookFrozenLoaderSchemaIdentityAndAvailableFields(t *testing.T) {
	for _, kind := range []string{"version", "space", "set", "nil_schema", "schema_id", "schema_space", "schema_set", "empty_key", "empty_name", "duplicate_key", "duplicate_name"} {
		t.Run(kind, func(t *testing.T) {
			f := newLoaderFixture()
			v := f.versions.value
			s := v.EvaluationSetSchema
			switch kind {
			case "version":
				v.ID++
			case "space":
				v.SpaceID++
			case "set":
				v.EvaluationSetID++
			case "nil_schema":
				v.EvaluationSetSchema = nil
			case "schema_id":
				s.ID = 0
			case "schema_space":
				s.SpaceID++
			case "schema_set":
				s.EvaluationSetID++
			case "empty_key":
				s.FieldSchemas[0].Key = ""
			case "empty_name":
				s.FieldSchemas[0].Name = ""
			case "duplicate_key":
				c := *s.FieldSchemas[0]
				c.Status = entity.FieldStatus_Deleted
				s.FieldSchemas = append(s.FieldSchemas, &c)
			case "duplicate_name":
				c := *s.FieldSchemas[0]
				c.Key = "another"
				s.FieldSchemas = append(s.FieldSchemas, &c)
			}
			out, err := f.load(t)
			require.ErrorIs(t, err, entity.ErrHookFrozenSchemaInvalid)
			require.Nil(t, out)
			require.Zero(t, f.items.batchCalls)
		})
	}
	f := newLoaderFixture()
	s := f.versions.value.EvaluationSetSchema
	s.FieldSchemas[0].Status = 0
	s.FieldSchemas = append(s.FieldSchemas, &entity.FieldSchema{Key: "deleted", Name: "question", Status: entity.FieldStatus_Deleted}, &entity.FieldSchema{Key: "sparse", Name: "optional", Status: entity.FieldStatus_Available})
	f.items.batch[0].Turns[0].FieldDataList = append(f.items.batch[0].Turns[0].FieldDataList, &entity.FieldData{Key: "deleted"}, &entity.FieldData{Key: "not-in-schema"})
	page, err := f.load(t)
	require.NoError(t, err)
	require.Len(t, page.Items[0].Item.Turns[0].FieldDataList, 1)
	require.Equal(t, "question", page.Items[0].Item.Turns[0].FieldDataList[0].Name)
	require.Len(t, f.items.batch[0].Turns[0].FieldDataList, 3)
	require.Zero(t, s.FieldSchemas[0].Status)
}

type frozenRequestAdapter struct {
	rpc.IDatasetRPCAdapter
	t                                    *testing.T
	batchCalls, versionCalls, draftCalls int
}

func (a *frozenRequestAdapter) BatchGetDatasetItems(ctx context.Context, p *rpc.BatchGetDatasetItemsParam) ([]*entity.EvaluationSetItem, error) {
	a.draftCalls++
	require.Nil(a.t, p.VersionID)
	require.Empty(a.t, p.ItemVersionQueries)
	require.Nil(a.t, p.Filter)
	require.Nil(a.t, p.TagFilter)
	return a.batch(p), nil
}
func (a *frozenRequestAdapter) BatchGetDatasetItemsByVersion(ctx context.Context, p *rpc.BatchGetDatasetItemsParam) ([]*entity.EvaluationSetItem, error) {
	a.batchCalls++
	require.Equal(a.t, int64(21), *p.VersionID)
	require.Empty(a.t, p.ItemVersionQueries)
	require.Nil(a.t, p.Filter)
	require.Nil(a.t, p.TagFilter)
	return a.batch(p), nil
}
func (a *frozenRequestAdapter) batch(p *rpc.BatchGetDatasetItemsParam) []*entity.EvaluationSetItem {
	require.LessOrEqual(a.t, len(p.ItemIDs), 100)
	items := make([]*entity.EvaluationSetItem, 0, len(p.ItemIDs))
	for i := len(p.ItemIDs) - 1; i >= 0; i-- {
		id := p.ItemIDs[i]
		items = append(items, &entity.EvaluationSetItem{ID: id + 100, SpaceID: p.SpaceID, EvaluationSetID: p.EvaluationSetID, ItemID: id, Turns: []*entity.Turn{{ID: 0, FieldDataList: []*entity.FieldData{{Key: "key-q", Content: &entity.Content{Text: gptr.Of("body")}}}}}})
	}
	return items
}
func (a *frozenRequestAdapter) GetDatasetItemVersion(ctx context.Context, space, set, item int64, version *int64, name *string) (*entity.EvaluationSetItemVersion, error) {
	a.versionCalls++
	require.Equal(a.t, int64(10), space)
	require.Equal(a.t, int64(20), set)
	require.Equal(a.t, int64(32), item)
	require.Equal(a.t, int64(7), *version)
	require.Nil(a.t, name)
	return &entity.EvaluationSetItemVersion{ItemID: item, ItemVersionID: *version, Turns: []*entity.Turn{{ID: 0, FieldDataList: []*entity.FieldData{{Key: "key-q", Content: &entity.Content{Text: gptr.Of("fixed")}}}}}}, nil
}

func requestSchema(space, set int64) *entity.EvaluationSetSchema {
	return &entity.EvaluationSetSchema{ID: set + 100, SpaceID: space, EvaluationSetID: set, FieldSchemas: []*entity.FieldSchema{{Key: "key-q", Name: fmt.Sprintf("question-%d", set), ContentType: entity.ContentTypeText, DefaultDisplayFormat: entity.FieldDisplayFormat_PlainText, Status: entity.FieldStatus_Available}}}
}
func (a *frozenRequestAdapter) GetDataset(ctx context.Context, space *int64, set int64, deleted *bool, shared *entity.SharedResourceOption) (*entity.EvaluationSet, error) {
	require.NotNil(a.t, space)
	require.True(a.t, *deleted)
	require.Nil(a.t, shared)
	return &entity.EvaluationSet{ID: set, SpaceID: *space, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: set, SpaceID: *space, EvaluationSetID: set, EvaluationSetSchema: requestSchema(*space, set)}}, nil
}
func (a *frozenRequestAdapter) GetDatasetVersion(ctx context.Context, space, version int64, deleted *bool, shared *entity.SharedResourceOption) (*entity.EvaluationSetVersion, *entity.EvaluationSet, error) {
	require.True(a.t, *deleted)
	require.Nil(a.t, shared)
	require.Equal(a.t, int64(10), space)
	require.Equal(a.t, int64(21), version)
	return &entity.EvaluationSetVersion{ID: version, SpaceID: space, EvaluationSetID: 20, EvaluationSetSchema: requestSchema(space, 20)}, nil, nil
}

func TestHookFrozenLoaderCrossSourcesAndOrdinal(t *testing.T) {
	f := newLoaderFixture()
	f.input.StartOrdinal = 5
	f.plans.page.Count = 7
	f.plans.page.NextOrdinal = 7
	f.plans.page.Items = []entity.HookPlanItem{{ID: 900, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: 21, ItemID: 30}, {ID: 901, SourceSpaceID: 11, EvalSetID: 22, EvalSetVersionID: 22, ItemID: 30}}
	a := &frozenRequestAdapter{t: t}
	f.deps.Items = &EvaluationSetItemServiceImpl{datasetRPCAdapter: a}
	f.deps.Versions = &EvaluationSetVersionServiceImpl{datasetRPCAdapter: a}
	f.deps.Sets = &EvaluationSetServiceImpl{datasetRPCAdapter: a}
	out, err := f.load(t)
	require.NoError(t, err)
	require.Len(t, out.Items, 2)
	require.Equal(t, int64(5), out.Items[0].Ordinal)
	require.Equal(t, int64(6), out.Items[1].Ordinal)
	require.Equal(t, int64(10), out.Items[0].Item.SpaceID)
	require.Equal(t, int64(11), out.Items[1].Item.SpaceID)
	require.Equal(t, "question-20", out.Items[0].Item.Turns[0].FieldDataList[0].Name)
	require.Equal(t, "question-22", out.Items[1].Item.Turns[0].FieldDataList[0].Name)
	require.Equal(t, 1, a.batchCalls)
	require.Equal(t, 1, a.draftCalls)
	out.Items[0].Frozen.ItemID = 1000
	require.Equal(t, int64(30), f.plans.page.Items[0].ItemID)
}

func TestHookFrozenLoaderBatchHundredBound(t *testing.T) {
	f := newLoaderFixture()
	f.plans.page.Items = nil
	for i := int64(1); i <= 100; i++ {
		f.plans.page.Items = append(f.plans.page.Items, entity.HookPlanItem{ID: 900 + i, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: 21, ItemID: i})
	}
	f.plans.page.Count = 100
	f.plans.page.NextOrdinal = 100
	a := &frozenRequestAdapter{t: t}
	f.deps.Items = &EvaluationSetItemServiceImpl{datasetRPCAdapter: a}
	out, err := f.load(t)
	require.NoError(t, err)
	require.Len(t, out.Items, 100)
	require.Equal(t, 1, a.batchCalls)
	for i, item := range out.Items {
		require.Equal(t, int64(i+1), item.Item.ItemID)
		require.Equal(t, int64(i), item.Ordinal)
	}
}

func TestHookFrozenLoaderCopiesMetadataAndMedia(t *testing.T) {
	f := newLoaderFixture()
	raw := f.items.batch[0]
	raw.BaseInfo = &entity.BaseInfo{CreatedBy: &entity.UserInfo{Name: gptr.Of("owner")}, CreatedAt: gptr.Of(int64(123))}
	raw.Tags = []*entity.ResourceTag{{TagName: "tag"}}
	raw.Turns[0].FieldDataList[0].Content = &entity.Content{ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{URL: gptr.Of("https://image.example"), StorageProvider: gptr.Of(entity.StorageProvider_ExternalUrl)}}
	out, err := f.load(t)
	require.NoError(t, err)
	copy := out.Items[0].Item
	*copy.BaseInfo.CreatedBy.Name = "changed"
	*copy.BaseInfo.CreatedAt = 999
	copy.Tags[0].TagName = "other"
	*copy.Turns[0].FieldDataList[0].Content.Image.URL = "changed"
	require.Equal(t, "owner", *raw.BaseInfo.CreatedBy.Name)
	require.Equal(t, int64(123), *raw.BaseInfo.CreatedAt)
	require.Equal(t, "tag", raw.Tags[0].TagName)
	require.Equal(t, "https://image.example", *raw.Turns[0].FieldDataList[0].Content.Image.URL)
}

func TestHookFrozenLoaderRejectsUnresolvedShapes(t *testing.T) {
	for _, content := range []*entity.Content{nil, {ContentType: gptr.Of(entity.ContentTypeText), ContentOmitted: gptr.Of(true)}, {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("short"), ContentOmitted: gptr.Of(true), FullContentBytes: gptr.Of(int32(20))}, {ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{ThumbURL: gptr.Of("thumbnail-only")}}, {ContentType: gptr.Of(entity.ContentTypeMultipart), ContentOmitted: gptr.Of(true), MultiPart: []*entity.Content{{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview")}}}, {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("valid"), MultiPart: []*entity.Content{omittedTree()}}} {
		f := newLoaderFixture()
		f.items.batch[0].Turns[0].FieldDataList[0].Content = content
		out, err := f.load(t)
		require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
		require.Nil(t, out)
	}
}

func TestHookFrozenLoaderRejectsNilDependencies(t *testing.T) {
	f := newLoaderFixture()
	f.deps.Plans = nil
	_, err := NewHookFrozenPlanLoader(f.deps)
	require.Error(t, err)
	f = newLoaderFixture()
	f.deps.Items = (*loaderItems)(nil)
	_, err = NewHookFrozenPlanLoader(f.deps)
	require.Error(t, err)
	f = newLoaderFixture()
	f.deps.Resolver = (*loaderResolver)(nil)
	_, err = NewHookFrozenPlanLoader(f.deps)
	require.Error(t, err)
}

func TestHookFrozenLoaderResolverReplyHasIndependentOwnership(t *testing.T) {
	f := newLoaderFixture()
	f.items.batch[0].Turns[0].FieldDataList[0].Content = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("preview"), ContentOmitted: gptr.Of(true)}
	resolved := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("full")}
	f.deps.Resolver = &loaderResolver{fn: func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error) {
		return resolved, nil
	}}
	out, err := f.load(t)
	require.NoError(t, err)
	*out.Items[0].Item.Turns[0].FieldDataList[0].Content.Text = "changed"
	require.Equal(t, "full", *resolved.Text)
}

func TestHookFrozenLoaderNullableEmptyText(t *testing.T) {
	f := newLoaderFixture()
	f.items.batch[0].Turns[0].FieldDataList[0].Content = &entity.Content{ContentType: gptr.Of(entity.ContentTypeText)}
	out, err := f.load(t)
	require.NoError(t, err)
	require.Equal(t, "", out.Items[0].Item.Turns[0].FieldDataList[0].Content.GetText())
	require.Nil(t, out.Items[0].Item.Turns[0].FieldDataList[0].Content.Text)
}

func TestHookFrozenLoaderLateSuccessAfterCancelIsNotReturned(t *testing.T) {
	f := newLoaderFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.deps.Items = loaderSlowItems{fn: func(context.Context) ([]*entity.EvaluationSetItem, error) { cancel(); return f.items.batch, nil }}
	l, err := NewHookFrozenPlanLoader(f.deps)
	require.NoError(t, err)
	out, err := l.LoadPage(ctx, f.input)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, out)
}

func TestHookFrozenLoaderUsesRealServiceRequestShapes(t *testing.T) {
	for _, version := range []int64{0, 20, 21} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			f := newLoaderFixture()
			f.plans.page.Count = 3
			f.plans.page.NextOrdinal = 3
			f.plans.page.Items = []entity.HookPlanItem{{ID: 900, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: version, ItemID: 30}, {ID: 901, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: version, ItemID: 32, ItemVersionID: 7}, {ID: 902, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: version, ItemID: 31}}
			a := &frozenRequestAdapter{t: t}
			f.deps.Items = &EvaluationSetItemServiceImpl{datasetRPCAdapter: a}
			page, err := f.load(t)
			require.NoError(t, err)
			require.Equal(t, []int64{30, 32, 31}, []int64{page.Items[0].Item.ItemID, page.Items[1].Item.ItemID, page.Items[2].Item.ItemID})
			require.Equal(t, 1, a.versionCalls)
			if version == 21 {
				require.Equal(t, 1, a.batchCalls)
				require.Zero(t, a.draftCalls)
				require.Equal(t, 1, f.versions.calls)
				require.Zero(t, f.sets.calls)
			} else {
				require.Equal(t, 1, a.draftCalls)
				require.Zero(t, a.batchCalls)
				require.Equal(t, 1, f.sets.calls)
				require.Zero(t, f.versions.calls)
			}
		})
	}
}

type loaderResolver struct {
	calls int
	fn    func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error)
}

func (r *loaderResolver) Resolve(ctx context.Context, item entity.HookPlanItem, turn int64, key string, c *entity.Content) (*entity.Content, error) {
	r.calls++
	return r.fn(ctx, item, turn, key, c)
}
func omittedTree() *entity.Content {
	return &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("cut"), ContentOmitted: gptr.Of(true), FullContentBytes: gptr.Of(int32(9)), FullContent: &entity.ObjectStorage{URI: gptr.Of("version-bound-object")}}}}}}
}

func TestHookFrozenLoaderRecursiveResolverIntegrity(t *testing.T) {
	for _, kind := range []string{"missing", "error", "nil", "still_partial", "wrong_type", "success"} {
		t.Run(kind, func(t *testing.T) {
			f := newLoaderFixture()
			f.items.batch[0].Turns[0].FieldDataList[0].Content = omittedTree()
			resolver := &loaderResolver{fn: func(ctx context.Context, item entity.HookPlanItem, turn int64, key string, c *entity.Content) (*entity.Content, error) {
				require.Equal(t, f.plans.page.Items[0], item)
				require.Zero(t, turn)
				require.Equal(t, "key-q", key)
				switch kind {
				case "error":
					return nil, errors.New("secret URI failure")
				case "nil":
					return nil, nil
				case "wrong_type":
					return &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("different")}, nil
				case "success":
					leaf := c.MultiPart[0].MultiPart[0]
					leaf.Text = gptr.Of("full body")
					leaf.ContentOmitted = gptr.Of(false)
				}
				return c, nil
			}}
			if kind != "missing" {
				f.deps.Resolver = resolver
			}
			out, err := f.load(t)
			if kind == "success" {
				require.NoError(t, err)
				require.Equal(t, "full body", *out.Items[0].Item.Turns[0].FieldDataList[0].Content.MultiPart[0].MultiPart[0].Text)
			} else {
				require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
				require.Nil(t, out)
				require.NotContains(t, err.Error(), "secret")
			}
			require.Equal(t, "cut", *f.items.batch[0].Turns[0].FieldDataList[0].Content.MultiPart[0].MultiPart[0].Text)
			if kind != "missing" {
				require.Equal(t, 1, resolver.calls)
			}
		})
	}
}

func TestHookFrozenLoaderEmptyTextMediaAndCycle(t *testing.T) {
	for _, content := range []*entity.Content{
		{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("")},
		{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("abc"), ContentOmitted: gptr.Of(false), FullContentBytes: gptr.Of(int32(3))},
		{ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{URL: gptr.Of("https://media.example/image")}, ContentOmitted: gptr.Of(false)},
		{ContentType: gptr.Of(entity.ContentTypeAudio), Audio: &entity.Audio{URI: gptr.Of("audio-object")}},
		{ContentType: gptr.Of(entity.ContentTypeVideo), Video: &entity.Video{URL: gptr.Of("https://media.example/video")}},
	} {
		f := newLoaderFixture()
		f.items.batch[0].Turns[0].FieldDataList[0].Content = content
		resolver := &loaderResolver{fn: func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error) {
			t.Fatal("complete content must not be resolved")
			return nil, nil
		}}
		f.deps.Resolver = resolver
		_, err := f.load(t)
		require.NoError(t, err)
		require.Zero(t, resolver.calls)
	}
	f := newLoaderFixture()
	cycle := &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart)}
	cycle.MultiPart = []*entity.Content{cycle}
	f.items.batch[0].Turns[0].FieldDataList[0].Content = cycle
	_, err := f.load(t)
	require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
}

type loaderSlowItems struct {
	EvaluationSetItemService
	fn func(context.Context) ([]*entity.EvaluationSetItem, error)
}

func (i loaderSlowItems) BatchGetEvaluationSetItems(ctx context.Context, _ *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	return i.fn(ctx)
}
func TestHookFrozenLoaderCancellationAndDependencyErrors(t *testing.T) {
	f := newLoaderFixture()
	l, err := NewHookFrozenPlanLoader(f.deps)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := l.LoadPage(ctx, f.input)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, out)
	require.Zero(t, f.plans.calls)
	f = newLoaderFixture()
	f.deps.Items = loaderSlowItems{fn: func(ctx context.Context) ([]*entity.EvaluationSetItem, error) { <-ctx.Done(); return nil, ctx.Err() }}
	l, err = NewHookFrozenPlanLoader(f.deps)
	require.NoError(t, err)
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	out, err = l.LoadPage(ctx, f.input)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, out)
	for _, where := range []string{"plan", "schema", "item"} {
		t.Run(where, func(t *testing.T) {
			f := newLoaderFixture()
			failure := errors.New("private source details")
			switch where {
			case "plan":
				f.plans.err = failure
			case "schema":
				f.versions.err = failure
			case "item":
				f.items.err = failure
			}
			out, err := f.load(t)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private")
			require.Nil(t, out)
		})
	}
}
