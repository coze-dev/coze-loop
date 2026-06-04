// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotificationFilterTreatsSystemTerminatedAsTerminated(t *testing.T) {
	conf := &ExptNotificationConf{
		Filter: &NotificationFilter{
			FilterConditions: []*NotificationFilterCondition{
				{
					Field:    &NotificationFilterField{FieldType: FieldType_ExptStatus},
					Operator: NotificationFilterOperatorType_In,
					Value:    strconv.FormatInt(int64(ExptStatus_Terminated), 10),
				},
			},
		},
	}

	assert.True(t, conf.MatchStatus(ExptStatus_Terminated))
	assert.True(t, conf.MatchStatus(ExptStatus_SystemTerminated))
	assert.False(t, conf.MatchStatus(ExptStatus_Success))
}
