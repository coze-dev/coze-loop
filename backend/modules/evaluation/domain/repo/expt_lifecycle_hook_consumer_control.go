// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Optional Hook-only writes keep existing consumer and DAO interfaces unchanged.
type IHookConsumerControlRepo interface {
	ApplyHookConsumerControl(context.Context, entity.HookConsumerControlInput) (entity.HookConsumerControlResult, error)
}
