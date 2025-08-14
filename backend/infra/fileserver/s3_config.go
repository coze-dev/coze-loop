// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package fileserver

import (
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

const (
	DefaultCacheSizeGT      = 64 << 10
	DefaultUploadPartSize   = 5 << 20
	DefaultDownloadPartSize = 10 << 20
	DefaultMaxObjectSize    = 4 << 30
	MinPartSize             = s3manager.MinUploadPartSize
	DefaultConcurrency      = 3
	DefaultSignTTL          = 24 * time.Hour
)

type S3Option func(*S3Config)

type S3Config struct {
	Endpoint        string `json:"endpoint" yaml:"endpoint"`
	Region          string `json:"region" yaml:"region"`
	Bucket          string `json:"bucket" yaml:"bucket" validate:"required"`
	AccessKeyID     string `json:"access_key_id" yaml:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" yaml:"secret_access_key"`

	// For download, if the size of the object is less than CacheSizeGT, it will be
	// read into memory, or else it will be downloaded to local file in parts.
	// Default is 64KB, must be greater than 0.
	CacheSizeGT int64 `json:"cache_size_gt" yaml:"cache_size_gt"`
	// For download, the size of each part. Default is 10MB, must be greater than 0.
	DownloadPartSize int64 `json:"download_part_size" yaml:"download_part_size"`
	// For upload, the size of each part. Default is 5MB, must be greater than 0.
	UploadPartSize int64 `json:"upload_part_size" yaml:"upload_part_size"`
	// The maximum size of the uploaded object. Default is 4GB, must be greater than
	// 0.
	MaxObjectSize int64 `json:"max_object_size" yaml:"max_object_size"`

	// If true, the path style will be used for the S3 endpoint.
	// If false, the virtual hosted-style will be used for the S3 endpoint.
	// Default is true.
	ForcePathStyle *bool `json:"force_path_style" yaml:"force_path_style"`
}

func NewS3Config(opts ...S3Option) *S3Config {
	cfg := &S3Config{
		CacheSizeGT:      DefaultCacheSizeGT,
		DownloadPartSize: DefaultDownloadPartSize,
		UploadPartSize:   DefaultUploadPartSize,
		MaxObjectSize:    DefaultMaxObjectSize,
		ForcePathStyle:   gptr.Of(true),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func (c *S3Config) Validate() error {
	// maybe replace with https://github.com/go-playground/validator/v10 later

	if c.Bucket == "" {
		return errors.New("bucket is required")
	}
	if c.CacheSizeGT <= 0 {
		return errors.New("cache_size_gt must be greater than 0")
	}
	if c.DownloadPartSize < MinPartSize {
		return errors.Errorf("download_part_size must be greater than %d", MinPartSize)
	}
	if c.UploadPartSize < MinPartSize {
		return errors.Errorf("upload_part_size must be greater than %d", MinPartSize)
	}
	if c.MaxObjectSize <= 0 {
		return errors.New("max_object_size must be greater than 0")
	}
	return nil
}
