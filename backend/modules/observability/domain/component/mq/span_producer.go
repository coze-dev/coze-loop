// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

//go:generate mockgen -destination=mocks/annotation_producer.go -package=mocks . IAnnotationProducer
type ISpanProducer interface {
	SendSpanWithAnnotation(ctx context.Context, message *entity.SpanEvent) error
}
