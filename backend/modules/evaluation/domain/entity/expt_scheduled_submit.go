// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// PreparedScheduledExpt contains server-prepared SQL data, never callback request fields.
// Resource authorization, ID allocation and encryption must finish before submission.
type PreparedScheduledExpt struct {
	Binding          ExptTemplateScheduleBinding
	TemplateRevision string
	Experiment       *Experiment
	Stats            *ExptStats
	Refs             []*ExptEvaluatorRef
	Mappings         []*ExptTurnResultFilterKeyMapping
	ConfigCipher     []byte
	Run              HookCreateRunInput
}
