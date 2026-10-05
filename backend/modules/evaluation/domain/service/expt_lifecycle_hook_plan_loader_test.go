// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type loaderPlans struct {
	repo.IHookPlanRepo
	page  *entity.HookPlanReadPage
	err   error
	calls int
}

func (p *loaderPlans) ReadPlanPage(context.Context, entity.HookPlanReadInput) (*entity.HookPlanReadPage, error) {
	p.calls++
	return p.page, p.err
}

type loaderItems struct {
	EvaluationSetItemService
	batch                    []*entity.EvaluationSetItem
	version                  *entity.EvaluationSetItemVersion
	batchCalls, versionCalls int
	err                      error
}

func (i *loaderItems) BatchGetEvaluationSetItems(context.Context, *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	i.batchCalls++
	return i.batch, i.err
}
func (i *loaderItems) GetEvaluationSetItemVersion(context.Context, int64, int64, int64, *int64, *string) (*entity.EvaluationSetItemVersion, error) {
	i.versionCalls++
	return i.version, i.err
}

type loaderVersions struct {
	EvaluationSetVersionService
	value *entity.EvaluationSetVersion
	err   error
	calls int
}

func (v *loaderVersions) GetEvaluationSetVersion(context.Context, int64, int64, *bool, *entity.SharedResourceOption) (*entity.EvaluationSetVersion, *entity.EvaluationSet, error) {
	v.calls++
	return v.value, nil, v.err
}

type loaderSets struct {
	IEvaluationSetService
	value *entity.EvaluationSet
	err   error
	calls int
}

func (s *loaderSets) GetEvaluationSet(context.Context, *int64, int64, *bool, *entity.SharedResourceOption) (*entity.EvaluationSet, error) {
	s.calls++
	return s.value, s.err
}

type loaderFixture struct {
	input    entity.HookPlanReadInput
	plans    *loaderPlans
	items    *loaderItems
	versions *loaderVersions
	sets     *loaderSets
	deps     HookFrozenPlanLoaderDependencies
}

func newLoaderFixture() *loaderFixture {
	input := entity.HookPlanReadInput{Key: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, ExecutionScope: "local", Limit: 100}
	tuple := entity.HookPlanItem{ID: 900, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: 21, ItemID: 30}
	schema := &entity.EvaluationSetSchema{ID: 40, SpaceID: 10, EvaluationSetID: 20, FieldSchemas: []*entity.FieldSchema{{Key: "key-q", Name: "question", ContentType: entity.ContentTypeText, DefaultDisplayFormat: entity.FieldDisplayFormat_PlainText, Status: entity.FieldStatus_Available}}}
	plans := &loaderPlans{page: &entity.HookPlanReadPage{Items: []entity.HookPlanItem{tuple}, NextOrdinal: 1, Count: 1, Hash: strings.Repeat("a", 64), Ready: true, RunVersion: 7}}
	items := &loaderItems{batch: []*entity.EvaluationSetItem{{ID: 130, ItemID: 30, SpaceID: 10, EvaluationSetID: 20, Turns: []*entity.Turn{{ID: 0, ItemID: 130, EvalSetID: 20, FieldDataList: []*entity.FieldData{{Key: "key-q", Content: &entity.Content{Text: gptr.Of("answer")}}}}}}}}
	versions := &loaderVersions{value: &entity.EvaluationSetVersion{ID: 21, SpaceID: 10, EvaluationSetID: 20, EvaluationSetSchema: schema}}
	sets := &loaderSets{value: &entity.EvaluationSet{ID: 20, SpaceID: 10, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: schema}}}
	return &loaderFixture{input: input, plans: plans, items: items, versions: versions, sets: sets, deps: HookFrozenPlanLoaderDependencies{Plans: plans, Items: items, Versions: versions, Sets: sets}}
}
func (f *loaderFixture) load(t *testing.T) (*entity.HookLoadedPlanPage, error) {
	t.Helper()
	l, err := NewHookFrozenPlanLoader(f.deps)
	require.NoError(t, err)
	return l.LoadPage(context.Background(), f.input)
}

func TestHookFrozenLoaderReturnsFrozenRowsAndSchemaNames(t *testing.T) {
	f := newLoaderFixture()
	page, err := f.load(t)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	got := page.Items[0]
	require.Equal(t, int64(0), got.Ordinal)
	require.Equal(t, f.plans.page.Items[0], got.Frozen)
	require.Equal(t, int64(130), got.Item.ID)
	require.Equal(t, "question", got.Item.Turns[0].FieldDataList[0].Name)
	require.Equal(t, entity.ContentTypeText, *got.Item.Turns[0].FieldDataList[0].Content.ContentType)
	require.Equal(t, "", f.items.batch[0].Turns[0].FieldDataList[0].Name)
}
