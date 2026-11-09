// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	evaluatormysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

// NewEvaluatorRecordDAO caches the first provider. Bind each test's real SQL
// session through the existing options; a repository transaction overrides it.
// The DAO conversion, query predicates and CAS implementation remain unchanged.
type legacyConsumerRecordDAO struct {
	evaluatormysql.EvaluatorRecordDAO
	session *gorm.DB
}

func (d *legacyConsumerRecordDAO) options(opts []db.Option) []db.Option {
	return append([]db.Option{db.WithTransaction(d.session)}, opts...)
}

func (d *legacyConsumerRecordDAO) CreateEvaluatorRecord(ctx context.Context, row *model.EvaluatorRecord, opts ...db.Option) error {
	return d.EvaluatorRecordDAO.CreateEvaluatorRecord(ctx, row, d.options(opts)...)
}

func (d *legacyConsumerRecordDAO) GetEvaluatorRecord(ctx context.Context, id int64, deleted bool, opts ...db.Option) (*model.EvaluatorRecord, error) {
	return d.EvaluatorRecordDAO.GetEvaluatorRecord(ctx, id, deleted, d.options(opts)...)
}

func (d *legacyConsumerRecordDAO) BatchGetEvaluatorRecord(ctx context.Context, ids []int64, deleted bool, opts ...db.Option) ([]*model.EvaluatorRecord, error) {
	return d.EvaluatorRecordDAO.BatchGetEvaluatorRecord(ctx, ids, deleted, d.options(opts)...)
}

func (d *legacyConsumerRecordDAO) CompareAndSwapEvaluatorRecordResult(ctx context.Context, id, space int64, from, to int8, score *float64, output string, opts ...db.Option) (int64, error) {
	return d.EvaluatorRecordDAO.CompareAndSwapEvaluatorRecordResult(ctx, id, space, from, to, score, output, d.options(opts)...)
}
