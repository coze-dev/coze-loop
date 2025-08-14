// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import "time"

type (
	CompressionMethod string
	Protocol          string
)

const (
	CompressionMethodLZ4     CompressionMethod = "lz4"
	CompressionMethodZSTD    CompressionMethod = "zstd"
	CompressionMethodGZIP    CompressionMethod = "gzip"
	CompressionMethodDeflate CompressionMethod = "deflate"
	CompressionMethodBrotli  CompressionMethod = "br"

	ProtocolHTTP   Protocol = "http"
	ProtocolNative Protocol = "native"
)

type Config struct {
	Host              string            `yaml:"host"`
	Database          string            `yaml:"database"`
	Username          string            `yaml:"username"`
	Password          string            `yaml:"password"`
	CompressionMethod CompressionMethod `yaml:"compression_method"`
	CompressionLevel  int               `yaml:"compressionLevel"`
	Protocol          Protocol          `yaml:"protocol"`
	DialTimeout       time.Duration     `yaml:"dialTimeout"`
	ReadTimeout       time.Duration     `yaml:"readTimeout"`
	Debug             bool              `yaml:"debug"`
	HttpHeaders       map[string]string `yaml:"http_headers"`
	Settings          map[string]any    `yaml:"setting"`
}
