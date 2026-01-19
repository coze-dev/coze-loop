// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertObjectStorageDO2DTO(t *testing.T) {
	t.Parallel()

	// nil 输入
	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, ConvertObjectStorageDO2DTO(nil))
	})

	// 完整字段映射
	t.Run("complete object storage", func(t *testing.T) {
		t.Parallel()
		input := &commonentity.ObjectStorage{
			Provider: gptr.Of(commonentity.StorageProvider_S3),
			Name:     gptr.Of("file.txt"),
			URI:      gptr.Of("oss://bucket/file.txt"),
			URL:      gptr.Of("https://example.com/file.txt"),
			ThumbURL: gptr.Of("https://example.com/thumb.png"),
		}
		result := ConvertObjectStorageDO2DTO(input)
		if assert.NotNil(t, result) {
			assert.Equal(t, dataset.StorageProvider(commonentity.StorageProvider_S3), gptr.Indirect(result.Provider))
			assert.Equal(t, gptr.Indirect(input.Name), gptr.Indirect(result.Name))
			assert.Equal(t, gptr.Indirect(input.URI), gptr.Indirect(result.URI))
			assert.Equal(t, gptr.Indirect(input.URL), gptr.Indirect(result.URL))
			assert.Equal(t, gptr.Indirect(input.ThumbURL), gptr.Indirect(result.ThumbURL))
		}
	})

	// Provider 为空时应当为 0 值指针
	t.Run("nil provider should become zero", func(t *testing.T) {
		t.Parallel()
		input := &commonentity.ObjectStorage{
			Name: gptr.Of("only-name"),
		}
		result := ConvertObjectStorageDO2DTO(input)
		if assert.NotNil(t, result) {
			assert.Equal(t, dataset.StorageProvider(0), gptr.Indirect(result.Provider))
			assert.Equal(t, "only-name", gptr.Indirect(result.Name))
			assert.Nil(t, result.URI)
			assert.Nil(t, result.URL)
			assert.Nil(t, result.ThumbURL)
		}
	})
}
