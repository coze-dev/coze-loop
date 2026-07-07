// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package validator 校验 SubmitExperimentOApi.notifications / UpdateExperimentOApi
// 入参的通知规则数组，落库前把 test_case 3 的 6 类非法输入挡在 API 层，避免脏行进
// experiment.notifications JSON 列。空数组视为显式禁用（test_case 4）直接返 nil，
// 是否 fallback PRD 默认由 caller 判断（NULL vs [] 由业务层锁定，本 validator 只保净）。
package validator

import (
	"errors"
	"net/url"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

const (
	// MaxWebhookURLLen tech_design 决策：webhook URL 上限 1024 字节（test_case 3.c）。
	MaxWebhookURLLen = 1024

	operatorContains    = "contains"
	operatorNotContains = "not_contains"
)

var (
	ErrInvalidWebhookURL = errors.New("invalid notification webhook url")
	ErrInvalidOperator   = errors.New("invalid notification operator")
	ErrInvalidTrigger    = errors.New("invalid notification trigger")
)

// Validate 校验通知规则数组；nil / [] 直接返 nil 允许 caller 走"显式禁用"或"回落默认"分支。
func Validate(rules []entity.NotificationRule) error {
	for i := range rules {
		if err := validateRule(&rules[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRule(r *entity.NotificationRule) error {
	switch r.Operator {
	case operatorContains, operatorNotContains:
	default:
		return ErrInvalidOperator
	}
	if len(r.Triggers) == 0 {
		return ErrInvalidTrigger
	}
	for _, t := range r.Triggers {
		if t < entity.NotificationTrigger_Started || t > entity.NotificationTrigger_Terminated {
			return ErrInvalidTrigger
		}
	}
	for i := range r.Actions {
		if err := validateAction(&r.Actions[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateAction(a *entity.NotificationAction) error {
	if a.Type != entity.NotificationActionType_Webhook {
		return nil
	}
	if a.URL == "" || len(a.URL) > MaxWebhookURLLen {
		return ErrInvalidWebhookURL
	}
	u, err := url.Parse(a.URL)
	if err != nil {
		return ErrInvalidWebhookURL
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrInvalidWebhookURL
	}
	if u.Host == "" {
		return ErrInvalidWebhookURL
	}
	return nil
}
