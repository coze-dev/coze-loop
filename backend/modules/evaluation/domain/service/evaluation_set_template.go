// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// EvaluationSetTemplateService is implemented by deployments that provide
// evaluation-set templates. The open-source deployment intentionally exposes
// an empty catalog and rejects template-based creation.
type EvaluationSetTemplateService interface {
	ListEvaluationSetTemplates(ctx context.Context, param *entity.ListEvaluationSetTemplatesParam) (templates []*entity.EvaluationSetTemplate, total *int64, nextPageToken *string, err error)
	BuildEvaluationSetSchemaFromTemplate(ctx context.Context, param *entity.BuildEvaluationSetSchemaFromTemplateParam) (*entity.EvaluationSetSchema, error)
	ValidateEvaluationSetSchemaUpdate(ctx context.Context, param *entity.ValidateEvaluationSetSchemaUpdateParam) error
}

type NoopEvaluationSetTemplateService struct{}

func NewNoopEvaluationSetTemplateService() EvaluationSetTemplateService {
	return &NoopEvaluationSetTemplateService{}
}

func (n *NoopEvaluationSetTemplateService) ListEvaluationSetTemplates(_ context.Context, _ *entity.ListEvaluationSetTemplatesParam) ([]*entity.EvaluationSetTemplate, *int64, *string, error) {
	total := int64(0)
	return []*entity.EvaluationSetTemplate{}, &total, nil, nil
}

func (n *NoopEvaluationSetTemplateService) BuildEvaluationSetSchemaFromTemplate(_ context.Context, _ *entity.BuildEvaluationSetSchemaFromTemplateParam) (*entity.EvaluationSetSchema, error) {
	return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluation set template is not supported"))
}

func (n *NoopEvaluationSetTemplateService) ValidateEvaluationSetSchemaUpdate(_ context.Context, _ *entity.ValidateEvaluationSetSchemaUpdateParam) error {
	return nil
}
