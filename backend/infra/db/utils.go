// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"fmt"
	"strings"

	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/gg/gvalue"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

type WhereBuilder struct {
	withIndex bool
	where     *clause.Where
}

func NewWhereBuilder() *WhereBuilder {
	return &WhereBuilder{where: new(clause.Where)}
}

func (b *WhereBuilder) EqOrIn(column string, values ...any) {
	if len(values) == 1 {
		b.where.Exprs = append(b.where.Exprs, &clause.Eq{
			Column: column,
			Value:  values[0],
		})
	}
	if len(values) > 1 {
		b.where.Exprs = append(b.where.Exprs, &clause.IN{
			Column: column,
			Values: values,
		})
	}
}

func (b *WhereBuilder) AddWhere(expr clause.Expression) {
	b.where.Exprs = append(b.where.Exprs, expr)
}

func (b *WhereBuilder) Build() (*clause.Where, error) {
	if !b.withIndex {
		return nil, errors.New("at least one of the query params using index must be set")
	}

	return b.where, nil
}

func (b *WhereBuilder) WithIndex() {
	b.withIndex = true
}

func WhereWithIndex(w *WhereBuilder) {
	w.WithIndex()
}

func MaybeAddEqToWhere[T comparable](b *WhereBuilder, fieldEq T, column string, opt ...func(builder *WhereBuilder)) {
	var zero T
	if fieldEq == zero {
		return
	}
	for _, o := range opt {
		o(b)
	}
	b.EqOrIn(column, fieldEq)
}

func MaybeAddInToWhere[T any](b *WhereBuilder, fieldIn []T, column string, opt ...func(builder *WhereBuilder)) {
	if len(fieldIn) == 0 {
		return
	}
	for _, o := range opt {
		o(b)
	}
	values := slices.Map(fieldIn, func(f T) any { return f })
	b.EqOrIn(column, values...)
}

func MaybeAddGtToWhere[T comparable](b *WhereBuilder, value T, column string, opt ...func(builder *WhereBuilder)) {
	var zero T
	if value == zero {
		return
	}
	for _, o := range opt {
		o(b)
	}
	b.AddWhere(&clause.Gt{Column: column, Value: value})
}

func MaybeAddLteToWhere[T comparable](b *WhereBuilder, value T, column string, opt ...func(builder *WhereBuilder)) {
	var zero T
	if value == zero {
		return
	}
	for _, o := range opt {
		o(b)
	}
	b.AddWhere(&clause.Lte{Column: column, Value: value})
}

// MaybeAddLikeToWhere 模糊搜索，字符串为空时不添加。会对原字符串中的通配符进行转义，如 % 转义为 \%。
func MaybeAddLikeToWhere(b *WhereBuilder, fieldLike string, column string, opt ...func(builder *WhereBuilder)) {
	if fieldLike == "" {
		return
	}
	for _, o := range opt {
		o(b)
	}
	b.where.Exprs = append(b.where.Exprs, &clause.Expr{
		SQL:  fmt.Sprintf("%s LIKE ? ESCAPE '\\\\'", column),
		Vars: []interface{}{"%" + escapeLikeWildcard(fieldLike) + "%"},
	})
}

// MaybeAddMultiLikeToWhere 某个字段多个模糊搜索条件.
func MaybeAddMultiLikeToWhere(b *WhereBuilder, fieldLikes []string, column string, opt ...func(builder *WhereBuilder)) {
	fieldLikes = gslice.Filter(fieldLikes, func(val string) bool {
		return gvalue.IsNotZero(val)
	})
	if len(fieldLikes) == 0 {
		return
	}
	for _, o := range opt {
		if o != nil {
			o(b)
		}
	}
	var sql string
	var vars []interface{}
	for i, fieldLike := range fieldLikes {
		sql = sql + fmt.Sprintf("%s LIKE ? ESCAPE '\\\\'", column)
		vars = append(vars, "%"+escapeLikeWildcard(fieldLike)+"%")
		if i != len(fieldLikes)-1 {
			sql = sql + " OR "
		}
	}
	b.where.Exprs = append(b.where.Exprs, &clause.Expr{
		SQL:  sql,
		Vars: vars,
	})
}

// escapeLikeWildcard 转义 Like 子句的通配符
func escapeLikeWildcard(s string) string {
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// RetryOnNotFound 如果 fn 返回 gorm.ErrRecordNotFound 错误，则使用主库重试一次。
// 用于处理主从同步延迟导致的读取不到数据的情况，注意 gorm.DB.Find 不会返回 ErrRecordNotFound 错误。
func RetryOnNotFound(fn func(opt ...Option) error, originalOpts []Option) error {
	retried := false
	opts := originalOpts
	for {
		err := fn(opts...)
		if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) || retried {
			return err
		}

		opt := &option{}
		for _, fn := range originalOpts {
			fn(opt)
		}
		if !opt.withMaster {
			opts = append(opts, WithMaster())
			retried = true
			continue
		}
		return err
	}
}
