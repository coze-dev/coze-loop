// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset_job"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestSourceTypeDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, SourceTypeDTO2DO(nil))
	})

	t.Run("file", func(t *testing.T) {
		t.Parallel()
		in := gptr.Of(dataset_job.SourceType_File)
		out := SourceTypeDTO2DO(in)
		if assert.NotNil(t, out) {
			assert.Equal(t, entity.SetSourceType(dataset_job.SourceType_File), *out)
		}
	})

	t.Run("dataset", func(t *testing.T) {
		t.Parallel()
		in := gptr.Of(dataset_job.SourceType_Dataset)
		out := SourceTypeDTO2DO(in)
		if assert.NotNil(t, out) {
			assert.Equal(t, entity.SetSourceType(dataset_job.SourceType_Dataset), *out)
		}
	})
}

func TestDatasetIOFileDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DatasetIOFileDTO2DO(nil))
	})

	t.Run("complete file", func(t *testing.T) {
		t.Parallel()
		format := gptr.Of(dataset_job.FileFormat_JSONL)
		compress := gptr.Of(dataset_job.FileFormat_ZIP)
		files := []string{"a.jsonl", "b.jsonl"}
		orig := gptr.Of("data.jsonl")
		dl := gptr.Of("https://example.com/data.jsonl")
		pid := gptr.Of("provider-id")
		pa := &dataset_job.ProviderAuth{ProviderAccountID: gptr.Of(int64(123))}
		in := &dataset_job.DatasetIOFile{
			Provider:         dataset.StorageProvider_S3,
			Path:             "oss://bucket/path",
			Format:           format,
			CompressFormat:   compress,
			Files:            files,
			OriginalFileName: orig,
			DownloadURL:      dl,
			ProviderID:       pid,
			ProviderAuth:     pa,
		}
		out := DatasetIOFileDTO2DO(in)
		if assert.NotNil(t, out) {
			assert.Equal(t, entity.StorageProvider(dataset.StorageProvider_S3), out.Provider)
			assert.Equal(t, "oss://bucket/path", out.Path)
			assert.Equal(t, files, out.Files)
			assert.Equal(t, gptr.Indirect(orig), gptr.Indirect(out.OriginalFileName))
			assert.Equal(t, gptr.Indirect(dl), gptr.Indirect(out.DownloadURL))
			assert.Equal(t, gptr.Indirect(pid), gptr.Indirect(out.ProviderID))
			if assert.NotNil(t, out.Format) {
			assert.Equal(t, entity.FileFormat(*format), *out.Format)
			}
			if assert.NotNil(t, out.CompressFormat) {
			assert.Equal(t, entity.FileFormat(*compress), *out.CompressFormat)
			}
			if assert.NotNil(t, out.ProviderAuth) {
			assert.Equal(t, gptr.Indirect(pa.ProviderAccountID), gptr.Indirect(out.ProviderAuth.ProviderAccountID))
			}
		}
	})
}

func TestDatasetIODatasetDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DatasetIODatasetDTO2DO(nil))
	})

	t.Run("complete dataset endpoint", func(t *testing.T) {
		t.Parallel()
		space := gptr.Of(int64(99))
		version := gptr.Of(int64(7))
		in := &dataset_job.DatasetIODataset{SpaceID: space, DatasetID: 12345, VersionID: version}
		out := DatasetIODatasetDTO2DO(in)
		if assert.NotNil(t, out) {
			assert.Equal(t, int64(12345), out.DatasetID)
			assert.Equal(t, gptr.Indirect(space), gptr.Indirect(out.SpaceID))
			assert.Equal(t, gptr.Indirect(version), gptr.Indirect(out.VersionID))
		}
	})
}

func TestProviderAuthDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, ProviderAuthDTO2DO(nil))
	})

	t.Run("maps provider account id", func(t *testing.T) {
		t.Parallel()
		in := &dataset_job.ProviderAuth{ProviderAccountID: gptr.Of(int64(456))}
		out := ProviderAuthDTO2DO(in)
		if assert.NotNil(t, out) {
			assert.Equal(t, int64(456), gptr.Indirect(out.ProviderAccountID))
		}
	})
}

func TestDatasetIOEndpointDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, DatasetIOEndpointDTO2DO(nil))
	})

	t.Run("endpoint with file and dataset", func(t *testing.T) {
		t.Parallel()
		in := &dataset_job.DatasetIOEndpoint{
			File: &dataset_job.DatasetIOFile{Provider: dataset.StorageProvider_TOS, Path: "tos://bucket/a"},
			Dataset: &dataset_job.DatasetIODataset{DatasetID: 1001},
		}
		out := DatasetIOEndpointDTO2DO(in)
		if assert.NotNil(t, out) {
			if assert.NotNil(t, out.File) {
				assert.Equal(t, entity.StorageProvider(dataset.StorageProvider_TOS), out.File.Provider)
				assert.Equal(t, "tos://bucket/a", out.File.Path)
			}
			if assert.NotNil(t, out.Dataset) {
				assert.Equal(t, int64(1001), out.Dataset.DatasetID)
			}
		}
	})
}

func TestFieldMappingDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, FieldMappingDTO2DO(nil))
	})

	t.Run("maps fields", func(t *testing.T) {
		t.Parallel()
		in := &dataset_job.FieldMapping{Source: "src", Target: "dst"}
		out := FieldMappingDTO2DO(in)
		if assert.NotNil(t, out) {
			assert.Equal(t, "src", out.Source)
			assert.Equal(t, "dst", out.Target)
		}
	})
}

func TestFieldMappingsDTO2DOs(t *testing.T) {
	t.Parallel()

	t.Run("nil slice", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, FieldMappingsDTO2DOs(nil))
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, FieldMappingsDTO2DOs([]*dataset_job.FieldMapping{}))
	})

	t.Run("single element", func(t *testing.T) {
		t.Parallel()
		in := []*dataset_job.FieldMapping{{Source: "a", Target: "b"}}
		out := FieldMappingsDTO2DOs(in)
		if assert.NotNil(t, out) && assert.Len(t, out, 1) {
			assert.Equal(t, "a", out[0].Source)
			assert.Equal(t, "b", out[0].Target)
		}
	})
}
