// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

type IItemCompletePublisher interface {
	PublishItemComplete(ctx context.Context, spaceID, exptID, itemID, exptRunID int64) error
}
