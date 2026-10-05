// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"time"
)

var (
	ErrScheduledRunTriggerInvalid  = errors.New("invalid scheduled run trigger")
	ErrScheduledRunTriggerConflict = errors.New("scheduled run trigger conflict")
	ErrScheduledRunTriggerMissing  = errors.New("scheduled run trigger missing")
)

const (
	ScheduledRunTriggerPending   = "pending"
	ScheduledRunTriggerSubmitted = "submitted"
)

// ScheduledRunTriggerIDs must be allocated before entering the repository transaction.
type ScheduledRunTriggerIDs struct {
	TriggerID    int64
	ExperimentID int64
	RunID        int64
}

// ScheduledRunTrigger is a durable submission receipt, not proof of callback/user authorization.
// Submitted means the local SQL submission committed, not that experiment execution completed.
type ScheduledRunTrigger struct {
	ID             int64
	BindingID      string
	BindingVersion int64
	InstanceID     string
	SpaceID        int64
	TemplateID     int64
	ExperimentID   int64
	RunID          int64
	Status         string
	CreatedAt      time.Time
}

func (ids ScheduledRunTriggerIDs) Validate() error {
	if ids.TriggerID <= 0 || ids.ExperimentID <= 0 || ids.RunID <= 0 {
		return ErrScheduledRunTriggerInvalid
	}
	return nil
}

func ValidateScheduledRunInstanceID(id string) error {
	if !scheduleBindingText(id, 128, false) {
		return ErrScheduledRunTriggerInvalid
	}
	return nil
}

func (t *ScheduledRunTrigger) Validate() error {
	if t == nil || t.ID <= 0 || t.BindingVersion <= 0 || t.SpaceID <= 0 || t.TemplateID <= 0 || t.ExperimentID <= 0 || t.RunID <= 0 ||
		!scheduleBindingText(t.BindingID, 128, false) || ValidateScheduledRunInstanceID(t.InstanceID) != nil || t.CreatedAt.IsZero() ||
		(t.Status != ScheduledRunTriggerPending && t.Status != ScheduledRunTriggerSubmitted) {
		return ErrScheduledRunTriggerInvalid
	}
	return nil
}
