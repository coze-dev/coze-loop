// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
)

// ILarkNotifier defines the interface for sending lark (feishu) notifications
type ILarkNotifier interface {
	// NotifyExperimentStatusChange sends a lark message about experiment status change
	NotifyExperimentStatusChange(ctx context.Context, spaceID int64, exptID int64, event string, creatorBy string) error
}

// NoopLarkNotifier is a no-op implementation of ILarkNotifier
type NoopLarkNotifier struct{}

// NewNoopLarkNotifier creates a new NoopLarkNotifier
func NewNoopLarkNotifier() ILarkNotifier {
	return &NoopLarkNotifier{}
}

// NotifyExperimentStatusChange does nothing (noop implementation)
func (n *NoopLarkNotifier) NotifyExperimentStatusChange(_ context.Context, _ int64, _ int64, _ string, _ string) error {
	return nil
}
