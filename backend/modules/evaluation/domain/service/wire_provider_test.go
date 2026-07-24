// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvideNilItemCompletePublisher(t *testing.T) {
	t.Parallel()

	got := ProvideNilItemCompletePublisher()
	assert.Nil(t, got, "open-source build should not wire an ItemCompletePublisher")
}
