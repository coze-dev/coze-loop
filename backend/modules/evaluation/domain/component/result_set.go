// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

// ResultSetAdmission is optional; deployments without it reject the new mode.
type ResultSetAdmission interface {
	CheckResultSetRuntime(ctx context.Context, spaceID int64, binaryVersion string) error
}
