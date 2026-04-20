// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
)

func TestStorageProvider_String(t *testing.T) {
	testCases := []struct {
		provider StorageProvider
		expect   string
	}{
		{StorageProvider_TOS, "TOS"},
		{StorageProvider_VETOS, "VETOS"},
		{StorageProvider_HDFS, "HDFS"},
		{StorageProvider_ImageX, "ImageX"},
		{StorageProvider_S3, "S3"},
		{StorageProvider_Abase, "Abase"},
		{StorageProvider_RDS, "RDS"},
		{StorageProvider_LocalFS, "LocalFS"},
		{StorageProvider_ExternalUrl, "ExternalUrl"},
		{StorageProvider(999), "<UNSET>"}, // 未知值
	}

	for _, tc := range testCases {
		t.Run(tc.expect, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.provider.String())
		})
	}
}

func TestStorageProviderFromString(t *testing.T) {
	testCases := []struct {
		input    string
		expect   StorageProvider
		expectOk bool
	}{
		{"TOS", StorageProvider_TOS, true},
		{"VETOS", StorageProvider_VETOS, true},
		{"HDFS", StorageProvider_HDFS, true},
		{"ImageX", StorageProvider_ImageX, true},
		{"S3", StorageProvider_S3, true},
		{"Abase", StorageProvider_Abase, true},
		{"RDS", StorageProvider_RDS, true},
		{"LocalFS", StorageProvider_LocalFS, true},
		{"ExternalUrl", StorageProvider_ExternalUrl, true},
		{"unknown", StorageProvider(0), false},
		{"", StorageProvider(0), false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			val, err := StorageProviderFromString(tc.input)
			if tc.expectOk {
				assert.NoError(t, err)
				assert.Equal(t, tc.expect, val)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestFileFormat_String(t *testing.T) {
	type fields struct {
		format FileFormat
		expect string
	}
	testCases := []fields{
		{FileFormat_JSONL, "JSONL"},
		{FileFormat_Parquet, "Parquet"},
		{FileFormat_CSV, "CSV"},
		{FileFormat_XLSX, "XLSX"},
		{FileFormat_ZIP, "ZIP"},
		{FileFormat(999), "<UNSET>"}, // 未知值
	}

	for _, tc := range testCases {
		t.Run(tc.expect, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.format.String())
		})
	}
}

func TestContent_IsContentOmitted(t *testing.T) {
	testCases := []struct {
		name   string
		c      *Content
		expect bool
	}{
		{
			name:   "nil content",
			c:      nil,
			expect: false,
		},
		{
			name:   "ContentOmitted is false",
			c:      &Content{ContentOmitted: gptr.Of(false)},
			expect: false,
		},
		{
			name:   "ContentOmitted is nil",
			c:      &Content{},
			expect: false,
		},
		{
			name: "ContentOmitted true, text type, full content matches text len",
			c: &Content{
				ContentOmitted:   gptr.Of(true),
				ContentType:      gptr.Of(ContentTypeText),
				Text:             gptr.Of("hello"),
				FullContentBytes: gptr.Of(int32(5)),
			},
			expect: false,
		},
		{
			name: "ContentOmitted true, text type, full content does not match text len",
			c: &Content{
				ContentOmitted:   gptr.Of(true),
				ContentType:      gptr.Of(ContentTypeText),
				Text:             gptr.Of("he"),
				FullContentBytes: gptr.Of(int32(100)),
			},
			expect: true,
		},
		{
			name: "ContentOmitted true, text type, FullContentBytes is zero",
			c: &Content{
				ContentOmitted: gptr.Of(true),
				ContentType:    gptr.Of(ContentTypeText),
				Text:           gptr.Of("hello"),
			},
			expect: true,
		},
		{
			name: "ContentOmitted true, non-text type",
			c: &Content{
				ContentOmitted: gptr.Of(true),
				ContentType:    gptr.Of(ContentTypeImage),
			},
			expect: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.c.IsContentOmitted())
		})
	}
}

func TestContent_GetText(t *testing.T) {
	testCases := []struct {
		name   string
		c      *Content
		expect string
	}{
		{
			name:   "nil content",
			c:      nil,
			expect: "",
		},
		{
			name:   "nil text",
			c:      &Content{},
			expect: "",
		},
		{
			name:   "has text",
			c:      &Content{Text: gptr.Of("hello")},
			expect: "hello",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.c.GetText())
		})
	}
}

func TestContent_SetText(t *testing.T) {
	testCases := []struct {
		name   string
		c      *Content
		text   string
		expect *string
	}{
		{
			name:   "nil content",
			c:      nil,
			text:   "hello",
			expect: nil,
		},
		{
			name:   "set text on valid content",
			c:      &Content{},
			text:   "world",
			expect: gptr.Of("world"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.c.SetText(tc.text)
			if tc.c != nil {
				assert.Equal(t, tc.expect, tc.c.Text)
			}
		})
	}
}

func TestContent_TextBytes(t *testing.T) {
	testCases := []struct {
		name   string
		c      *Content
		expect int
	}{
		{
			name:   "nil content",
			c:      nil,
			expect: 0,
		},
		{
			name:   "nil text",
			c:      &Content{},
			expect: 0,
		},
		{
			name:   "has text",
			c:      &Content{Text: gptr.Of("hello")},
			expect: 5,
		},
		{
			name:   "empty text",
			c:      &Content{Text: gptr.Of("")},
			expect: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.c.TextBytes())
		})
	}
}

func TestContent_GetContentType(t *testing.T) {
	testCases := []struct {
		name   string
		c      *Content
		expect ContentType
	}{
		{
			name:   "nil content",
			c:      nil,
			expect: "",
		},
		{
			name:   "nil content type",
			c:      &Content{},
			expect: "",
		},
		{
			name:   "has content type",
			c:      &Content{ContentType: gptr.Of(ContentTypeText)},
			expect: ContentTypeText,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.c.GetContentType())
		})
	}
}

func TestContent_SetContentType(t *testing.T) {
	testCases := []struct {
		name        string
		c           *Content
		contentType ContentType
		expect      *ContentType
	}{
		{
			name:        "nil content",
			c:           nil,
			contentType: ContentTypeText,
			expect:      nil,
		},
		{
			name:        "set content type",
			c:           &Content{},
			contentType: ContentTypeImage,
			expect:      gptr.Of(ContentTypeImage),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.c.SetContentType(tc.contentType)
			if tc.c != nil {
				assert.Equal(t, tc.expect, tc.c.ContentType)
			}
		})
	}
}

func TestBaseInfo_GetCreatedBy(t *testing.T) {
	user := &UserInfo{Name: gptr.Of("alice")}
	testCases := []struct {
		name   string
		do     *BaseInfo
		expect *UserInfo
	}{
		{
			name:   "nil created_by",
			do:     &BaseInfo{},
			expect: nil,
		},
		{
			name:   "has created_by",
			do:     &BaseInfo{CreatedBy: user},
			expect: user,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.do.GetCreatedBy())
		})
	}
}

func TestBaseInfo_SetCreatedBy(t *testing.T) {
	user := &UserInfo{Name: gptr.Of("bob")}
	do := &BaseInfo{}
	do.SetCreatedBy(user)
	assert.Equal(t, user, do.CreatedBy)
}

func TestBaseInfo_GetUpdatedBy(t *testing.T) {
	user := &UserInfo{Name: gptr.Of("carol")}
	testCases := []struct {
		name   string
		do     *BaseInfo
		expect *UserInfo
	}{
		{
			name:   "nil updated_by",
			do:     &BaseInfo{},
			expect: nil,
		},
		{
			name:   "has updated_by",
			do:     &BaseInfo{UpdatedBy: user},
			expect: user,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.do.GetUpdatedBy())
		})
	}
}

func TestBaseInfo_SetUpdatedBy(t *testing.T) {
	user := &UserInfo{Name: gptr.Of("dave")}
	do := &BaseInfo{}
	do.SetUpdatedBy(user)
	assert.Equal(t, user, do.UpdatedBy)
}

func TestBaseInfo_SetUpdatedAt(t *testing.T) {
	ts := int64(1234567890)
	do := &BaseInfo{}
	do.SetUpdatedAt(&ts)
	assert.Equal(t, &ts, do.UpdatedAt)
}

func TestModelConfig_GetModelID(t *testing.T) {
	testCases := []struct {
		name   string
		m      *ModelConfig
		expect int64
	}{
		{
			name:   "nil model config",
			m:      nil,
			expect: 0,
		},
		{
			name:   "nil model id",
			m:      &ModelConfig{},
			expect: 0,
		},
		{
			name:   "has model id",
			m:      &ModelConfig{ModelID: gptr.Of(int64(42))},
			expect: 42,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.m.GetModelID())
		})
	}
}

func TestStorageProviderPtr(t *testing.T) {
	p := StorageProvider_TOS
	result := StorageProviderPtr(p)
	assert.NotNil(t, result)
	assert.Equal(t, p, *result)
}
