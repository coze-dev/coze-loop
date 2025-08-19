// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package pagination

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
)

const (
	ColumnUpdatedAt = "updated_at"
	ColumnCreatedAt = "created_at"
	ColumnID        = "id"
)

const DefaultLimit = 10

// Paginator 支持分页查询，默认按 `order by id desc limit 10` 排序
// - 排序字段 [WithOrderBy]
// - 支持按 offset 或 cursor 分页, 同时提供时，按 cursor 分页
type Paginator struct {
	timeColumn string
	idColumn   string
	asc        bool
	limit      int
	offset     int
	rawCursor  string
	err        error
	cursor     *Cursor
	result     *PageResult
}

type PaginatorOption func(*Paginator)

// WithOrderBy 指定排序字段，默认按 `id` 字段，支持:
// 1. 单个字段: 需为 (*)Int 类型且有唯一性，如 `id`
// 2. 双字段: 第一个为 (*)time.Time 类型，第二个为 (*)Int 类型且有唯一性，如 `updated_at, id`
func WithOrderBy(columns ...string) PaginatorOption {
	return func(p *Paginator) {
		switch len(columns) {
		case 1:
			p.idColumn = columns[0]
		case 2:
			p.timeColumn = columns[0]
			p.idColumn = columns[1]
		default:
		}
	}
}

// WithPage 指定页码和每页条数，按 offset 读取，用于数据量较小的场景
func WithPage(page, pageSize int32) PaginatorOption {
	return func(p *Paginator) {
		if pageSize > 0 {
			p.limit = int(pageSize)
		}
		if page > 1 {
			p.offset = int(page-1) * int(pageSize)
		}
	}
}

// WithCursor 指定游标，用于数据量较大的场景。cursor 由 [GetResult.Cursor] 返回
func WithCursor(cursor string) PaginatorOption {
	return func(p *Paginator) {
		p.rawCursor = cursor
	}
}

// WithOrderByAsc 指定排序方向，默认降序
func WithOrderByAsc(isAsc bool) PaginatorOption {
	return func(p *Paginator) {
		p.asc = isAsc
	}
}

// WithLimit 指定每页条数，默认 10 条
func WithLimit(limit int) PaginatorOption {
	return func(p *Paginator) {
		if limit > 0 {
			p.limit = limit
		}
	}
}

// WithError 指定错误，用于在构建 Paginator 时返回错误
func WithError(err error) PaginatorOption {
	return func(p *Paginator) {
		p.err = err
	}
}

func WithPrePage(page, pageSize *int32, cursor *string) PaginatorOption {
	return func(p *Paginator) {
		WithPage(gptr.Indirect(page), gptr.Indirect(pageSize))(p)
		WithCursor(gptr.Indirect(cursor))(p)
	}
}

func New(opts ...PaginatorOption) *Paginator {
	p := &Paginator{
		idColumn: ColumnID,
		limit:    DefaultLimit,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Find safe for nil pagination.
func (p *Paginator) Find(ctx context.Context, tx *gorm.DB, dest any, conds ...any) *gorm.DB {
	if p == nil {
		return tx.Find(dest, conds...)
	}
	if err := p.build(); err != nil {
		_ = tx.AddError(errno.InvalidParamErr(err, "invalid paginator"))
		return tx
	}

	// todo: count total
	defer goroutine.Recovery(ctx)

	if p.cursor != nil || p.offset == 0 {
		return p.findByCursor(tx, dest, conds...)
	}
	return p.findByOffset(tx, dest, conds...)
}

// Result safe for nil pagination, always return a non-nil PageResult.
func (p *Paginator) Result() *PageResult {
	if p == nil || p.result == nil {
		return &PageResult{}
	}
	return p.result
}

func (p *Paginator) build() error {
	if p.err != nil {
		return p.err
	}

	if p.timeColumn == "" && p.idColumn == "" {
		return errors.New("both time_column and id_column are empty")
	}

	if p.rawCursor != "" {
		c, err := decodeCursor(p.rawCursor)
		if err != nil {
			return errors.New("decode cursor")
		}
		p.cursor = c
	}
	return nil
}

func (p *Paginator) findByCursor(tx *gorm.DB, dest any, conds ...any) *gorm.DB {
	result := p.addCursorToQuery(tx).
		Limit(p.limit+1). // to calculate next cursor
		Order(p.orderBy()).
		Find(dest, conds...)

	if result.Error != nil {
		return result
	}

	pr := &PageResult{}
	p.result = pr

	dv := reflect.Indirect(reflect.ValueOf(dest))
	if dv.Kind() != reflect.Slice || dv.Len() <= p.limit {
		return result
	}

	dv.SetLen(p.limit)
	p.setNextCursor(result, pr, dv)
	return tx
}

func (p *Paginator) addCursorToQuery(tx *gorm.DB) *gorm.DB {
	if p.cursor == nil {
		return tx
	}

	c := p.cursor
	newWhere := func(expr ...clause.Expression) *clause.Where { return &clause.Where{Exprs: expr} }
	newComp := func(col string, value any) clause.Expression {
		if p.asc {
			return &clause.Gt{Column: col, Value: value}
		}
		return &clause.Lt{Column: col, Value: value}
	}

	switch {
	case c.TimeValue != nil && c.IDValue != nil: // order by ${timeColumn}, ${idColumn}
		e1 := &clause.AndConditions{
			Exprs: []clause.Expression{
				&clause.Eq{Column: p.timeColumn, Value: c.TimeValue},
				newComp(p.idColumn, c.IDValue),
			},
		}
		e2 := newComp(p.timeColumn, c.TimeValue)
		return tx.Where(newWhere(clause.Or(e1, e2)))

	case c.IDValue != nil: // order by ${idColumn}
		return tx.Where(newWhere(newComp(p.idColumn, c.IDValue)))

	default:
		return tx
	}
}

func (p *Paginator) findByOffset(db *gorm.DB, dest any, conds ...any) *gorm.DB {
	return db.Order(p.orderBy()).Limit(p.limit).Offset(p.offset).Find(dest, conds...)
}

func (p *Paginator) orderBy() string {
	suffix := " desc"
	if p.asc {
		suffix = " asc"
	}

	switch { //nolint:staticcheck
	case p.timeColumn == "":
		return p.idColumn + suffix
	default:
		return fmt.Sprintf("%s %s, %s %s", p.timeColumn, suffix, p.idColumn, suffix)
	}
}

func (p *Paginator) setNextCursor(tx *gorm.DB, pr *PageResult, sliceV reflect.Value) {
	if tx.Statement == nil || tx.Statement.Schema == nil {
		return
	}
	last := reflect.Indirect(sliceV.Index(sliceV.Len() - 1))
	if last.Kind() != reflect.Struct {
		return
	}

	name2Field := tx.Statement.Schema.FieldsByDBName
	next := &Cursor{}

	if f, ok := name2Field[p.idColumn]; ok {
		v := reflect.Indirect(last.FieldByName(f.Name))
		if v.CanInt() {
			next.IDValue = gptr.Of(v.Int())
		}
	}

	if f, ok := name2Field[p.timeColumn]; ok {
		v := reflect.Indirect(last.FieldByName(f.Name))
		if v.CanInterface() {
			if timeVal, ok := v.Interface().(time.Time); ok {
				next.TimeValue = gptr.Of(timeVal)
			}
		}
	}

	cursor, err := next.Encode()
	if err != nil {
		_ = tx.AddError(errors.WithMessage(err, "encode next cursor"))
	}
	pr.Cursor = cursor
}

func (p *Paginator) GetErr() error {
	return p.err
}
