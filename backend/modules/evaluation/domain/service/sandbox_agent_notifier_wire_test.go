// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvideNoSandboxAgentNotifiers(t *testing.T) {
	got := ProvideNoSandboxAgentNotifiers()

	assert.Empty(t, got)
}
