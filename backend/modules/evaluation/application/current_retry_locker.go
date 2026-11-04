// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"github.com/rs/xid"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

func NewManualRetryLockerFactory(client redis.Cmdable) service.ManualRetryLockerFactory {
	return func() lock.ILocker {
		return lock.NewRedisLockerWithHolder(client, lock.LifecycleCleanupHolderPrefix+xid.New().String())
	}
}
