// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"github.com/google/wire"
)

// StorageSet 评测记录大对象存储相关依赖，供 experimentSet 等使用
var StorageSet = wire.NewSet(
	NewRecordDataStorage,
)
