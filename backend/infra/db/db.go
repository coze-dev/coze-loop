// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/utils"
	"gorm.io/plugin/dbresolver"
)

type Option func(option *option)

//go:generate mockgen -destination=mocks/db.go -package=mocks . Provider
type Provider interface {
	// NewSession 创建一个新的数据库会话
	NewSession(ctx context.Context, opts ...Option) *gorm.DB

	// Transaction 执行一个事务
	Transaction(ctx context.Context, fc func(tx *gorm.DB) error, opts ...Option) error
}

// WithMaster 强制读主库
func WithMaster() Option {
	return func(option *option) {
		option.withMaster = true
	}
}

// WithTransaction 使用一个已有的事务
func WithTransaction(tx *gorm.DB) Option {
	return func(option *option) {
		option.tx = tx
	}
}

// Debug 启用调试模式
func Debug() Option {
	return func(option *option) {
		option.debug = true
	}
}

// WithDeleted 返回软删的数据
func WithDeleted() Option {
	return func(option *option) {
		option.withDeleted = true
	}
}

func WithSelectForUpdate() Option {
	return func(config *option) {
		config.forUpdate = true
	}
}

type option struct {
	tx          *gorm.DB
	debug       bool
	withMaster  bool
	withDeleted bool
	forUpdate   bool
}

// provider 包装 gorm.db 并强制提供 ctx 以串联 trace
type provider struct {
	db *gorm.DB
}

var _ Provider = &provider{}

// NewDB 创建一个 db 实例
func NewDB(dialer gorm.Dialector, opts ...gorm.Option) (Provider, error) {
	db, err := gorm.Open(dialer, opts...)
	if err != nil {
		return nil, err
	}
	return &provider{db: db}, nil
}

// NewDBFromConfig 从配置创建一个 db 实例
func NewDBFromConfig(cfg *Config, opts ...gorm.Option) (Provider, error) {
	if !utils.Contains(mysql.UpdateClauses, "RETURNING") {
		mysql.UpdateClauses = append(mysql.UpdateClauses, "RETURNING")
	}
	// Known issue: this option will make the opts using gorm.Config not working.
	opts = append(opts, &gorm.Config{
		TranslateError: true,
	})

	db, err := gorm.Open(mysql.Open(cfg.buildDSN()), opts...)
	if err != nil {
		return nil, err
	}

	return &provider{db: db}, nil
}

func (p *provider) NewSession(ctx context.Context, opts ...Option) *gorm.DB {
	session := p.db

	opt := &option{}
	for _, fn := range opts {
		fn(opt)
	}
	if opt.tx != nil {
		session = opt.tx
	}
	if opt.debug {
		session = session.Debug()
	}
	if opt.withMaster {
		session = session.Clauses(dbresolver.Write)
	}
	if opt.withDeleted {
		session = session.Unscoped()
	}
	if opt.forUpdate {
		session = session.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return session.WithContext(ctx)
}

func (p *provider) Transaction(ctx context.Context, fc func(tx *gorm.DB) error, opts ...Option) error {
	session := p.NewSession(ctx, opts...)
	return session.Transaction(fc)
}

func ContainWithMasterOpt(opt []Option) bool {
	o := &option{}
	for _, fn := range opt {
		fn(o)
		if o.withMaster {
			return true
		}
	}
	return false
}
