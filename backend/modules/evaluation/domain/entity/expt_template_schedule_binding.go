// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrExptTemplateScheduleBindingInvalid  = errors.New("invalid template schedule binding")
	ErrExptTemplateScheduleBindingCorrupt  = errors.New("corrupt template schedule binding")
	ErrExptTemplateScheduleBindingConflict = errors.New("template schedule binding conflict")
	ErrExptTemplateScheduleBindingMissing  = errors.New("template schedule binding missing")
)

type ExptTemplateScheduleBindingKey struct {
	SpaceID        int64
	TemplateID     int64
	ExecutionScope string
}

type ExptTemplateScheduleCallback struct {
	PSM       string `json:"psm"`
	Method    string `json:"method"`
	Cluster   string `json:"cluster"`
	IDLBranch string `json:"idl_branch,omitempty"`
	VRegion   string `json:"v_region,omitempty"`
}

// ExptTemplateScheduleBinding is server-owned persistence, not proof of authorization.
type ExptTemplateScheduleBinding struct {
	SchemaVersion  int                          `json:"schema_version"`
	BindingID      string                       `json:"binding_id"`
	Version        int64                        `json:"version"`
	UserID         string                       `json:"user_id"`
	IdentityType   string                       `json:"identity_type"`
	SpaceID        int64                        `json:"space_id"`
	TemplateID     int64                        `json:"template_id"`
	ExecutionScope string                       `json:"execution_scope"`
	Namespace      string                       `json:"namespace"`
	Group          string                       `json:"group"`
	BizKey         string                       `json:"biz_key"`
	JobID          string                       `json:"job_id"`
	Enabled        bool                         `json:"enabled"`
	BoundAt        time.Time                    `json:"bound_at"`
	Callback       ExptTemplateScheduleCallback `json:"callback"`
}

// ExptTemplateScheduleReceipt must come from the server-side scheduler adapter.
type ExptTemplateScheduleReceipt struct {
	JobID     string
	Namespace string
	Group     string
	BizKey    string
	Callback  ExptTemplateScheduleCallback
}

func (k ExptTemplateScheduleBindingKey) Validate() error {
	if k.SpaceID <= 0 || k.TemplateID <= 0 || !scheduleBindingText(k.ExecutionScope, 128, false) {
		return ErrExptTemplateScheduleBindingInvalid
	}
	return nil
}

func (c ExptTemplateScheduleCallback) Validate() error {
	if !scheduleBindingText(c.PSM, 256, false) || !scheduleBindingText(c.Method, 128, false) ||
		!scheduleBindingText(c.Cluster, 128, false) || !scheduleBindingText(c.IDLBranch, 256, true) || !scheduleBindingText(c.VRegion, 128, true) {
		return ErrExptTemplateScheduleBindingInvalid
	}
	return nil
}

func (b *ExptTemplateScheduleBinding) Validate() error {
	if b == nil || b.SchemaVersion != 1 || b.Version <= 0 || b.IdentityType != "fornax_user" ||
		!scheduleBindingText(b.BindingID, 128, false) || !scheduleBindingUserID(b.UserID) ||
		!scheduleBindingText(b.JobID, 128, true) || !scheduleBindingText(b.Namespace, 128, false) ||
		!scheduleBindingText(b.Group, 128, false) || !scheduleBindingText(b.BizKey, 256, false) ||
		b.BoundAt.Unix() <= 0 || b.BoundAt.Year() > 9999 {
		return ErrExptTemplateScheduleBindingInvalid
	}
	if err := (ExptTemplateScheduleBindingKey{b.SpaceID, b.TemplateID, b.ExecutionScope}).Validate(); err != nil {
		return err
	}
	return b.Callback.Validate()
}

// Active only describes persisted registration state, not permission to execute.
func (b *ExptTemplateScheduleBinding) Active() bool {
	return b.Validate() == nil && b.Enabled && b.JobID != ""
}

func (r ExptTemplateScheduleReceipt) Validate() error {
	if !scheduleBindingText(r.JobID, 128, false) || !scheduleBindingText(r.Namespace, 128, false) ||
		!scheduleBindingText(r.Group, 128, false) || !scheduleBindingText(r.BizKey, 256, false) {
		return ErrExptTemplateScheduleBindingInvalid
	}
	return r.Callback.Validate()
}

func scheduleBindingText(s string, limit int, optional bool) bool {
	if len(s) > limit || (!optional && s == "") {
		return false
	}
	for i := range s {
		if s[i] < 33 || s[i] > 126 {
			return false
		}
	}
	return true
}

func scheduleBindingUserID(id string) bool {
	if len(id) == 0 || len(id) > 128 || id == "0" || !utf8.ValidString(id) {
		return false
	}
	for _, c := range id {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return false
		}
	}
	return true
}
