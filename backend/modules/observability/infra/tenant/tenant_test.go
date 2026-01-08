// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tenant

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/stretchr/testify/assert"
)

func TestCorner(t *testing.T) {
	opt := new(tenant.Option)
	optFns := []tenant.OptFn{
		tenant.WithWorkspaceID(213),
	}
	for _, optFn := range optFns {
		optFn(opt)
	}
	assert.Equal(t, int64(213), opt.WorkspaceID)
}
