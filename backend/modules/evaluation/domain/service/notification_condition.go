// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// exptStatusToConditionValue maps ExptStatus enum to the condition value string
// used in NotificationCondition.Values.
func exptStatusToConditionValue(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Processing:
		return "processing"
	case entity.ExptStatus_Success:
		return "success"
	case entity.ExptStatus_Failed:
		return "failed"
	case entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return "terminated"
	default:
		return ""
	}
}

// matchesCondition evaluates whether the given experiment status matches the
// notification condition. If config is nil, the default config is used.
func matchesCondition(config *entity.NotificationConfig, status entity.ExptStatus) bool {
	if config == nil {
		config = entity.DefaultNotificationConfig()
	}

	cond := config.Condition
	if cond == nil {
		// No condition means always match (backward compat).
		return true
	}

	statusValue := exptStatusToConditionValue(status)
	if statusValue == "" {
		// Unknown status never matches.
		return false
	}

	inValues := contains(cond.Values, statusValue)

	switch cond.Operator {
	case "in":
		return inValues
	case "not_in":
		return !inValues
	default:
		// Unknown operator: default to "in" semantics.
		return inValues
	}
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
