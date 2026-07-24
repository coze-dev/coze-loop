// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCKFromConfig_ConnPoolSettings(t *testing.T) {
	t.Run("with MaxOpenConns and MaxIdleConns", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:9000",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolNative,
			DialTimeout:  1 * time.Millisecond,
			MaxOpenConns: 20,
			MaxIdleConns: 10,
		}
		_, _ = NewCKFromConfig(cfg)
	})

	t.Run("zero values keep defaults", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:9000",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolNative,
			DialTimeout:  1 * time.Millisecond,
			MaxOpenConns: 0,
			MaxIdleConns: 0,
		}
		_, _ = NewCKFromConfig(cfg)
	})

	t.Run("http protocol with conn pool", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:8123",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolHTTP,
			DialTimeout:  1 * time.Millisecond,
			MaxOpenConns: 50,
			MaxIdleConns: 25,
		}
		_, _ = NewCKFromConfig(cfg)
	})

	t.Run("with compression", func(t *testing.T) {
		cfg := &Config{
			Host:              "127.0.0.1:9000",
			Database:          "default",
			Username:          "default",
			Protocol:          ProtocolNative,
			DialTimeout:       1 * time.Millisecond,
			CompressionMethod: CompressionMethodLZ4,
			CompressionLevel:  3,
			MaxOpenConns:      10,
			MaxIdleConns:      5,
		}
		p, err := NewCKFromConfig(cfg)
		if err == nil {
			assert.NotNil(t, p)
		}
	})
}
