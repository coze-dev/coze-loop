// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTimeIntervals(t *testing.T) {
	//assert.Equal(t,
	//	NewTimeIntervals(1760425397156, 1760508194371,
	//		MetricGranularity1Day), []string{"1760371200000", "1760457600000"})
	//assert.Equal(t,
	//	NewTimeIntervals(1760425397156, 1760508194371,
	//		MetricGranularity1Hour), []string{"1760428800000", "1760432400000", "1760439600000", "1760443200000", "1760446800000", "1760450400000", "1760497200000"})
	assert.Equal(t,
		NewTimeIntervals(1760425397156, 1760508194371,
			MetricGranularity1Week), []string{"1760371200000", "1760457600000"})
}
