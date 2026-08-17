// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvideSandboxAgentNotifiers(t *testing.T) {
	notifier := &sandboxAgentNotifier{}

	got := ProvideSandboxAgentNotifiers(notifier)

	require.Len(t, got, 1)
	assert.Same(t, notifier, got[0])
}
