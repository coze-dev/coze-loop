// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"

	std_ck "github.com/ClickHouse/clickhouse-go/v2"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

type Provider interface {
	NewSession(ctx context.Context) *gorm.DB
}

type provider struct {
	db *gorm.DB
}

func (p *provider) NewSession(ctx context.Context) *gorm.DB {
	return p.db.WithContext(ctx)
}

func NewCKFromConfig(cfg *Config) (Provider, error) {
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
	switch cfg.CompressionMethod {
	case CompressionMethodLZ4:
		opt.Compression = &std_ck.Compression{
			Method: std_ck.CompressionLZ4,
			Level:  cfg.CompressionLevel,
		}
	case CompressionMethodZSTD:
		opt.Compression = &std_ck.Compression{
			Method: std_ck.CompressionZSTD,
			Level:  cfg.CompressionLevel,
		}
	case CompressionMethodGZIP:
		opt.Compression = &std_ck.Compression{
			Method: std_ck.CompressionGZIP,
			Level:  cfg.CompressionLevel,
		}
	case CompressionMethodDeflate:
		opt.Compression = &std_ck.Compression{
			Method: std_ck.CompressionDeflate,
			Level:  cfg.CompressionLevel,
		}
	case CompressionMethodBrotli:
		opt.Compression = &std_ck.Compression{
			Method: std_ck.CompressionBrotli,
			Level:  cfg.CompressionLevel,
		}
	}
	switch cfg.Protocol {
	case ProtocolHTTP:
		opt.Protocol = std_ck.HTTP
	case ProtocolNative:
		opt.Protocol = std_ck.Native
	}
	ckSqlDB := std_ck.OpenDB(opt)
	ckDb, err := gorm.Open(clickhouse.New(clickhouse.Config{Conn: ckSqlDB}))
	if err != nil {
		return nil, err
	}
	return &provider{db: ckDb}, nil
}
