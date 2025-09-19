// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/infra/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InsertAnnotationParam struct {
	Table       string
	Annotations []*model.ObservabilityAnnotation
}

type GetAnnotationParam struct {
	Tables    []string
	ID        string
	StartTime int64 // us
	EndTime   int64 // us
	Limit     int32
}

type ListAnnotationsParam struct {
	Tables          []string
	SpanIDs         []string
	StartTime       int64 // us
	EndTime         int64 // us
	DescByUpdatedAt bool
	Limit           int32
}

//go:generate mockgen -destination=mocks/annotation_dao.go -package=mocks . IAnnotationDao
type IAnnotationDao interface {
	Insert(context.Context, *InsertAnnotationParam) error
	Get(context.Context, *GetAnnotationParam) (*model.ObservabilityAnnotation, error)
	List(context.Context, *ListAnnotationsParam) ([]*model.ObservabilityAnnotation, error)
}

func NewAnnotationCkDaoImpl(db ck.Provider) (IAnnotationDao, error) {
	return &AnnotationCkDaoImpl{
		db: db,
	}, nil
}

type AnnotationCkDaoImpl struct {
	db ck.Provider
}

func (a *AnnotationCkDaoImpl) Insert(ctx context.Context, params *InsertAnnotationParam) error {
	if params == nil || len(params.Annotations) == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	db := a.db.NewSession(ctx)
	retryTimes := 3
	var lastErr error
	for i := 0; i < retryTimes; i++ {
		if err := db.Table(params.Table).Create(params.Annotations).Error; err != nil {
			lastErr = err
		} else {
			return nil
		}
	}
	return errorx.WrapByCode(lastErr, obErrorx.CommercialCommonInternalErrorCodeCode)
}

func (a *AnnotationCkDaoImpl) Get(ctx context.Context, params *GetAnnotationParam) (*model.ObservabilityAnnotation, error) {
	if params == nil || params.ID == "" {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	db, err := a.buildSql(ctx, &annoSqlParam{
		Tables:    params.Tables,
		StartTime: params.StartTime,
		EndTime:   params.EndTime,
		ID:        params.ID,
		Limit:     1,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "Get Annotation SQL: %s", db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(nil)
	}))
	var annotations []*model.ObservabilityAnnotation
	if err := db.Find(&annotations).Error; err != nil {
		return nil, err
	}
	if len(annotations) == 0 {
		return nil, nil
	} else if len(annotations) > 1 {
		logs.CtxWarn(ctx, "multiple annotations found")
	}
	return annotations[0], nil
}

func (a *AnnotationCkDaoImpl) List(ctx context.Context, params *ListAnnotationsParam) ([]*model.ObservabilityAnnotation, error) {
	if params == nil || len(params.SpanIDs) == 0 {
		return nil, nil
	}
	db, err := a.buildSql(ctx, &annoSqlParam{
		Tables:          params.Tables,
		StartTime:       params.StartTime,
		EndTime:         params.EndTime,
		SpanIDs:         params.SpanIDs,
		DescByUpdatedAt: params.DescByUpdatedAt,
		Limit:           params.Limit,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "List Annotations SQL: %s", db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(nil)
	}))
	var annotations []*model.ObservabilityAnnotation
	if err := db.Find(&annotations).Error; err != nil {
		return nil, err
	}
	return annotations, nil
}

type annoSqlParam struct {
	Tables          []string
	StartTime       int64
	EndTime         int64
	ID              string
	SpanIDs         []string
	DescByUpdatedAt bool
	Limit           int32
}

func (a *AnnotationCkDaoImpl) buildSql(ctx context.Context, param *annoSqlParam) (*gorm.DB, error) {
	db := a.db.NewSession(ctx)
	var tableQueries []*gorm.DB
	for _, table := range param.Tables {
		query, err := a.buildSingleSql(ctx, db, table, param)
		if err != nil {
			return nil, err
		}
		tableQueries = append(tableQueries, query)
	}
	if len(tableQueries) == 0 {
		return nil, fmt.Errorf("no table configured")
	} else if len(tableQueries) == 1 {
		query := tableQueries[0].ToSQL(func(tx *gorm.DB) *gorm.DB {
			return tx.Find(nil)
		})
		query += " SETTINGS final = 1"
		return db.Raw(query), nil
	} else {
		queries := make([]string, 0)
		for i := 0; i < len(tableQueries); i++ {
			query := tableQueries[i].ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find(nil)
			})
			queries = append(queries, "("+query+")")
		}
		sql := fmt.Sprintf("SELECT * FROM (%s)", strings.Join(queries, " UNION ALL "))
		if param.DescByUpdatedAt {
			sql += " ORDER BY updated_at DESC"
		} else {
			sql += " ORDER BY created_at ASC"
		}
		sql += fmt.Sprintf(" LIMIT %d SETTINGS final = 1", param.Limit)
		return db.Raw(sql), nil
	}
}

// buildSingleSql 构建单表查询SQL
func (a *AnnotationCkDaoImpl) buildSingleSql(ctx context.Context, db *gorm.DB, tableName string, param *annoSqlParam) (*gorm.DB, error) {
	sqlQuery := db.
		Table(tableName).
		Where("deleted_at = 0")

	if param.ID != "" {
		sqlQuery = sqlQuery.Where("id = ?", param.ID)
	}
	if len(param.SpanIDs) > 0 {
		sqlQuery = sqlQuery.Where("span_id IN (?)", param.SpanIDs)
	}
	sqlQuery = sqlQuery.
		Where("start_time >= ?", param.StartTime).
		Where("start_time <= ?", param.EndTime)
	if param.DescByUpdatedAt {
		sqlQuery = sqlQuery.Order(clause.OrderBy{Columns: []clause.OrderByColumn{
			{Column: clause.Column{Name: "updated_at"}, Desc: true},
		}})
	} else {
		sqlQuery = sqlQuery.Order(clause.OrderBy{Columns: []clause.OrderByColumn{
			{Column: clause.Column{Name: "created_at"}, Desc: false},
		}})
	}
	sqlQuery = sqlQuery.Limit(int(param.Limit))
	return sqlQuery, nil
}
