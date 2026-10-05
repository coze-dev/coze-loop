// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IHookBoundConsumerRepo interface {
	ReadBoundConsumerItem(context.Context, entity.HookRunKey, string, int64) (*entity.HookBoundConsumerItem, error)
}
