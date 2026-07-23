// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"testing"
	"time"

	std_ck "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
)

func TestNewCKFromConfig_ConnPoolSettings(t *testing.T) {
	t.Run("MaxOpenConns and MaxIdleConns are applied", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:9000",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolNative,
			DialTimeout:  3 * time.Second,
			MaxOpenConns: 20,
			MaxIdleConns: 10,
		}
		opt := buildOptions(cfg)
		db := std_ck.OpenDB(opt)
		defer func() { _ = db.Close() }()

		if cfg.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MaxIdleConns)
		}

		assert.Equal(t, 20, db.Stats().MaxOpenConnections)
	})

	t.Run("zero values keep defaults", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:9000",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolNative,
			DialTimeout:  3 * time.Second,
			MaxOpenConns: 0,
			MaxIdleConns: 0,
		}
		opt := buildOptions(cfg)
		db := std_ck.OpenDB(opt)
		defer func() { _ = db.Close() }()

		if cfg.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MaxIdleConns)
		}

		assert.Equal(t, 0, db.Stats().MaxOpenConnections)
	})

	t.Run("only MaxOpenConns set", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:9000",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolNative,
			MaxOpenConns: 50,
			MaxIdleConns: 0,
		}
		opt := buildOptions(cfg)
		db := std_ck.OpenDB(opt)
		defer func() { _ = db.Close() }()

		if cfg.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MaxIdleConns)
		}

		assert.Equal(t, 50, db.Stats().MaxOpenConnections)
	})

	t.Run("only MaxIdleConns set", func(t *testing.T) {
		cfg := &Config{
			Host:         "127.0.0.1:9000",
			Database:     "default",
			Username:     "default",
			Protocol:     ProtocolNative,
			MaxOpenConns: 0,
			MaxIdleConns: 15,
		}
		opt := buildOptions(cfg)
		db := std_ck.OpenDB(opt)
		defer func() { _ = db.Close() }()

		if cfg.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MaxIdleConns)
		}

		assert.Equal(t, 0, db.Stats().MaxOpenConnections)
	})
}

func buildOptions(cfg *Config) *std_ck.Options {
	opt := &std_ck.Options{
		Addr: []string{cfg.Host},
		Auth: std_ck.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
		Debug:       cfg.Debug,
		HttpHeaders: cfg.HttpHeaders,
		Settings:    cfg.Settings,
	}
	switch cfg.Protocol {
	case ProtocolHTTP:
		opt.Protocol = std_ck.HTTP
	case ProtocolNative:
		opt.Protocol = std_ck.Native
	}
	return opt
}
