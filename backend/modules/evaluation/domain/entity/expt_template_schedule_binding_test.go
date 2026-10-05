// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExptTemplateScheduleBindingValidation(t *testing.T) {
	valid := ExptTemplateScheduleBinding{
		SchemaVersion: 1, BindingID: "binding-1", Version: 1, UserID: "user-1", IdentityType: "fornax_user",
		SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: "template.10.20",
		Enabled: true, BoundAt: time.Unix(100, 0).UTC(),
		Callback: ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"},
	}
	require.NoError(t, valid.Validate())
	require.False(t, valid.Active(), "pending registration must not be executable")
	active := valid
	active.JobID = "job-1"
	require.NoError(t, active.Validate())
	t.Run("registered binding is active", func(t *testing.T) { require.True(t, active.Active()) })
	active.Enabled = false
	require.False(t, active.Active(), "a retained receipt must not enable a disabled binding")
	for _, tc := range []struct {
		name   string
		mutate func(*ExptTemplateScheduleBinding)
	}{
		{"schema", func(b *ExptTemplateScheduleBinding) { b.SchemaVersion = 2 }},
		{"binding", func(b *ExptTemplateScheduleBinding) { b.BindingID = "" }},
		{"version", func(b *ExptTemplateScheduleBinding) { b.Version = 0 }},
		{"user", func(b *ExptTemplateScheduleBinding) { b.UserID = "" }},
		{"identity", func(b *ExptTemplateScheduleBinding) { b.IdentityType = "service" }},
		{"space", func(b *ExptTemplateScheduleBinding) { b.SpaceID = 0 }},
		{"template", func(b *ExptTemplateScheduleBinding) { b.TemplateID = -1 }},
		{"scope", func(b *ExptTemplateScheduleBinding) { b.ExecutionScope = "other\n" }},
		{"namespace", func(b *ExptTemplateScheduleBinding) { b.Namespace = "" }},
		{"group", func(b *ExptTemplateScheduleBinding) { b.Group = "" }},
		{"biz key", func(b *ExptTemplateScheduleBinding) { b.BizKey = "" }},
		{"job size", func(b *ExptTemplateScheduleBinding) { b.JobID = strings.Repeat("x", 129) }},
		{"time", func(b *ExptTemplateScheduleBinding) { b.BoundAt = time.Time{} }},
		{"callback", func(b *ExptTemplateScheduleBinding) { b.Callback.Method = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := valid
			tc.mutate(&b)
			require.ErrorIs(t, b.Validate(), ErrExptTemplateScheduleBindingInvalid)
			require.False(t, b.Active())
		})
	}
	var absent *ExptTemplateScheduleBinding
	require.ErrorIs(t, absent.Validate(), ErrExptTemplateScheduleBindingInvalid)
	require.False(t, absent.Active())
}

func TestExptTemplateScheduleBindingUserIDContract(t *testing.T) {
	for _, tc := range []struct {
		name, id string
		valid    bool
	}{
		{"unicode opaque", "tenant:用户-α", true},
		{"beyond int64", "9223372036854775808", true},
		{"leading zero opaque", "00", true},
		{"format character", "user\u200bname", true},
		{"ascii 128 bytes", strings.Repeat("u", 128), true},
		{"unicode 128 bytes", strings.Repeat("界", 42) + "ab", true},
		{"empty", "", false},
		{"zero", "0", false},
		{"invalid utf8", "user\xff", false},
		{"ascii 129 bytes", strings.Repeat("u", 129), false},
		{"unicode 129 bytes", strings.Repeat("界", 43), false},
		{"leading space", " user", false},
		{"trailing space", "user ", false},
		{"unicode nbsp", "user\u00a0name", false},
		{"unicode em space", "user\u2003name", false},
		{"unicode narrow nbsp", "user\u202fname", false},
		{"unicode control", "user\u009fname", false},
		{"newline", "user\nname", false},
		{"tab", "user\tname", false},
		{"nul", "user\x00name", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := ExptTemplateScheduleBinding{
				SchemaVersion: 1, BindingID: "binding-1", Version: 1, UserID: tc.id, IdentityType: "fornax_user",
				SpaceID: 10, TemplateID: 20, ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: "template.10.20",
				Enabled: true, JobID: "job-1", BoundAt: time.Unix(100, 0).UTC(),
				Callback: ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"},
			}
			if tc.valid {
				require.NoError(t, b.Validate())
			} else {
				require.ErrorIs(t, b.Validate(), ErrExptTemplateScheduleBindingInvalid)
			}
			require.Equal(t, tc.valid, b.Active())
			require.Equal(t, tc.id, b.UserID, "validation must not normalize opaque identity")
		})
	}
}
