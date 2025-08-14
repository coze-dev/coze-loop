// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

//go:generate mockgen -destination=mocks/annotation_producer.go -package=mocks . IAnnotationProducer
type IAnnotationProducer interface {
	SendAnnotation(ctx context.Context, message *entity.AnnotationEvent) error
}
