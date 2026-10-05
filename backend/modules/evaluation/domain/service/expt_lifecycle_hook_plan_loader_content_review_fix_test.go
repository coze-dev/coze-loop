// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func contentReviewText(omitted bool) *entity.Content {
	return &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("fake"), ContentOmitted: gptr.Of(omitted), FullContentBytes: gptr.Of(int32(4))}
}

func TestHookFrozenContentReviewRawOmitted(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content *entity.Content
	}{
		{"equal_text", contentReviewText(true)},
		{"image_url", &entity.Content{ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{URL: gptr.Of("https://media.example/image")}, ContentOmitted: gptr.Of(true)}},
		{"audio_uri", &entity.Content{ContentType: gptr.Of(entity.ContentTypeAudio), Audio: &entity.Audio{URI: gptr.Of("bound-audio")}, ContentOmitted: gptr.Of(true)}},
		{"video_uri", &entity.Content{ContentType: gptr.Of(entity.ContentTypeVideo), Video: &entity.Video{URI: gptr.Of("bound-video")}, ContentOmitted: gptr.Of(true)}},
		{"nested", &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{contentReviewText(true)}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLoaderFixture()
			f.items.batch[0].Turns[0].FieldDataList[0].Content = tc.content
			out, err := f.load(t)
			require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
			require.Nil(t, out)
		})
	}
}

func TestHookFrozenContentReviewResolverCannotBeBypassed(t *testing.T) {
	f := newLoaderFixture()
	original := contentReviewText(true)
	f.items.batch[0].Turns[0].FieldDataList[0].Content = original
	r := &loaderResolver{fn: func(_ context.Context, p entity.HookPlanItem, turn int64, key string, c *entity.Content) (*entity.Content, error) {
		require.Equal(t, f.plans.page.Items[0], p)
		require.Zero(t, turn)
		require.Equal(t, "key-q", key)
		require.True(t, *c.ContentOmitted)
		c.ContentOmitted = gptr.Of(false)
		c.Text = gptr.Of("real body")
		return c, nil
	}}
	f.deps.Resolver = r
	out, err := f.load(t)
	require.NoError(t, err)
	require.Equal(t, 1, r.calls)
	require.Equal(t, "real body", out.Items[0].Item.Turns[0].FieldDataList[0].Content.GetText())
	require.True(t, *original.ContentOmitted)
	require.Equal(t, "fake", original.GetText())
}

func TestHookFrozenContentReviewResolverStillOmitted(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			f := newLoaderFixture()
			input := contentReviewText(true)
			input.FullContentBytes = gptr.Of(int32(9))
			if nested {
				input = &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{input}}
			}
			f.items.batch[0].Turns[0].FieldDataList[0].Content = input
			returned := contentReviewText(true)
			if nested {
				returned = &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart), MultiPart: []*entity.Content{returned}}
			}
			f.deps.Resolver = &loaderResolver{fn: func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error) {
				return returned, nil
			}}
			out, err := f.load(t)
			require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
			require.Nil(t, out)
		})
	}
}

func TestHookFrozenContentReviewSizeIsOnlyHint(t *testing.T) {
	for _, size := range []int32{-1, 0, 2, 999999} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			f := newLoaderFixture()
			c := contentReviewText(false)
			c.FullContentBytes = gptr.Of(size)
			f.items.batch[0].Turns[0].FieldDataList[0].Content = c
			out, err := f.load(t)
			require.NoError(t, err)
			got := out.Items[0].Item.Turns[0].FieldDataList[0].Content
			require.Equal(t, "fake", got.GetText())
			require.Equal(t, size, *got.FullContentBytes)
		})
	}
}

func TestHookFrozenContentReviewMessageListAfterResolution(t *testing.T) {
	for _, tc := range []struct {
		name             string
		schema           entity.SchemaKey
		body             string
		omitted, wantErr bool
	}{
		{"unresolved", entity.SchemaKey_MessageList, `[{"role":"user","content":"<attachment#asset-1>"}]`, true, true},
		{"escaped_token", entity.SchemaKey_MessageList, `[{"role":"user","content":"\u003cattachment#asset-1\u003e"}]`, true, true},
		{"content_parts", entity.SchemaKey_MessageList, `[{"role":"user","content":[{"type":"image_url","image_url":{"url":"<attachment#asset-1>"}}]}]`, true, true},
		{"resolved", entity.SchemaKey_MessageList, `[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://media.example/image"}},{"type":"text","text":"real text"}]}]`, true, false},
		{"plain_text_literal", entity.SchemaKey_String, `[{"role":"user","content":"<attachment#asset-1>"}]`, true, false},
		{"unrelated_literal", entity.SchemaKey_MessageList, `[{"role":"user","content":"attachment#asset-1 and <attachment#>","name":"<attachment#not-content>"}]`, true, false},
		{"empty_list", entity.SchemaKey_MessageList, `[]`, true, false},
		{"already_inline", entity.SchemaKey_MessageList, `[{"role":"user","content":"<attachment#literal>"}]`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLoaderFixture()
			f.versions.value.EvaluationSetSchema.FieldSchemas[0].SchemaKey = gptr.Of(tc.schema)
			input := contentReviewText(tc.omitted)
			input.FullContentBytes = gptr.Of(int32(100))
			input.MultiPart = []*entity.Content{{ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{Name: gptr.Of("not-a-part-key.png"), URL: gptr.Of("https://media.example/image")}}}
			if !tc.omitted {
				input.Text = gptr.Of(tc.body)
				input.FullContentBytes = nil
			}
			f.items.batch[0].Turns[0].FieldDataList[0].Content = input
			resolver := &loaderResolver{fn: func(_ context.Context, _ entity.HookPlanItem, _ int64, _ string, c *entity.Content) (*entity.Content, error) {
				c.Text = gptr.Of(tc.body)
				c.ContentOmitted = gptr.Of(false)
				c.FullContentBytes = nil
				return c, nil
			}}
			f.deps.Resolver = resolver
			out, err := f.load(t)
			if tc.wantErr {
				require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
				require.Nil(t, out)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.body, out.Items[0].Item.Turns[0].FieldDataList[0].Content.GetText())
			}
			if tc.omitted {
				require.Equal(t, 1, resolver.calls)
				require.Equal(t, "fake", input.GetText())
				require.True(t, *input.ContentOmitted)
			} else {
				require.Zero(t, resolver.calls)
			}
		})
	}
}

func TestHookFrozenContentReviewURIRemainsGeneralReference(t *testing.T) {
	for _, kind := range []entity.ContentType{entity.ContentTypeImage, entity.ContentTypeAudio, entity.ContentTypeVideo} {
		t.Run(string(kind), func(t *testing.T) {
			f := newLoaderFixture()
			c := &entity.Content{ContentType: gptr.Of(kind)}
			switch kind {
			case entity.ContentTypeImage:
				c.Image = &entity.Image{URI: gptr.Of("bound/image")}
			case entity.ContentTypeAudio:
				c.Audio = &entity.Audio{URI: gptr.Of("bound/audio")}
			case entity.ContentTypeVideo:
				c.Video = &entity.Video{URI: gptr.Of("bound/video")}
			}
			f.items.batch[0].Turns[0].FieldDataList[0].Content = c
			out, err := f.load(t)
			require.NoError(t, err)
			require.NotNil(t, out)
		})
	}
}

func TestHookFrozenContentReviewFailureReturnsNoPartialPage(t *testing.T) {
	f := newLoaderFixture()
	f.versions.value.EvaluationSetSchema.FieldSchemas[0].SchemaKey = gptr.Of(entity.SchemaKey_MessageList)
	second := f.plans.page.Items[0]
	second.ID = 901
	second.ItemID = 31
	f.plans.page.Items = append(f.plans.page.Items, second)
	f.plans.page.Count = 2
	f.plans.page.NextOrdinal = 2
	f.items.batch = append(f.items.batch, &entity.EvaluationSetItem{ID: 131, ItemID: 31, SpaceID: 10, EvaluationSetID: 20, Turns: []*entity.Turn{{ID: 0, FieldDataList: []*entity.FieldData{{Key: "key-q", Content: contentReviewText(true)}}}}})
	f.deps.Resolver = &loaderResolver{fn: func(_ context.Context, tuple entity.HookPlanItem, _ int64, _ string, c *entity.Content) (*entity.Content, error) {
		require.Equal(t, int64(31), tuple.ItemID)
		c.ContentOmitted = gptr.Of(false)
		c.Text = gptr.Of(`[{"content":"<attachment#still-missing>"}]`)
		return c, nil
	}}
	out, err := f.load(t)
	require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
	require.Nil(t, out)
	require.Equal(t, "", f.items.batch[0].Turns[0].FieldDataList[0].Name)
	require.True(t, *f.items.batch[1].Turns[0].FieldDataList[0].Content.ContentOmitted)
}

func TestHookFrozenContentReviewCancellationDuringResolution(t *testing.T) {
	f := newLoaderFixture()
	input := contentReviewText(true)
	f.items.batch[0].Turns[0].FieldDataList[0].Content = input
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.deps.Resolver = &loaderResolver{fn: func(_ context.Context, _ entity.HookPlanItem, _ int64, _ string, c *entity.Content) (*entity.Content, error) {
		c.ContentOmitted = gptr.Of(false)
		cancel()
		return c, nil
	}}
	l, err := NewHookFrozenPlanLoader(f.deps)
	require.NoError(t, err)
	out, err := l.LoadPage(ctx, f.input)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, out)
	require.True(t, *input.ContentOmitted)
}

func TestHookFrozenContentReviewInvalidResolvedMessageList(t *testing.T) {
	for _, body := range []string{`{"content":"not a list"}`, `[null]`, `[{"content":`} {
		f := newLoaderFixture()
		f.versions.value.EvaluationSetSchema.FieldSchemas[0].SchemaKey = gptr.Of(entity.SchemaKey_MessageList)
		f.items.batch[0].Turns[0].FieldDataList[0].Content = contentReviewText(true)
		f.deps.Resolver = &loaderResolver{fn: func(_ context.Context, _ entity.HookPlanItem, _ int64, _ string, c *entity.Content) (*entity.Content, error) {
			c.ContentOmitted = gptr.Of(false)
			c.Text = gptr.Of(body)
			return c, nil
		}}
		out, err := f.load(t)
		require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
		require.Nil(t, out)
	}
}
