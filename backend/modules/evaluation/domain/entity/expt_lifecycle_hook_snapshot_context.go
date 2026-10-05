// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

// ValidateHookRunContext validates SPI shape, not the authority of its supplier.
func ValidateHookRunContext(c *spi.HookRunContext, phase HookPhase) error {
	invalid := errors.New("invalid hook context")
	if c == nil || !validHookSnapshotID(c.WorkspaceID) || !validHookSnapshotID(c.ExperimentID) || !validHookSnapshotID(c.RunID) {
		return invalid
	}
	switch c.GetRunMode() {
	case spi.HookRunModeSubmit, spi.HookRunModeFailRetry, spi.HookRunModeAppend, spi.HookRunModeRetryAll, spi.HookRunModeRetryItems, spi.HookRunModeTrialRun:
	default:
		return invalid
	}
	u := c.Initiator
	if u == nil || !validHookSnapshotText(u.UserID, 128, true) || u.GetIdentityType() != "fornax_user" || !validHookSnapshotText(u.Email, 320, false) || !validHookSnapshotText(u.Name, 256, false) {
		return invalid
	}
	e := c.Experiment
	if e == nil || !validHookSnapshotText(e.Name, 512, true) || (e.GetType() != "offline" && e.GetType() != "online") || c.EvalSets == nil {
		return invalid
	}
	for _, set := range c.EvalSets {
		if set == nil || !validHookSnapshotID(set.WorkspaceID) || !validHookSnapshotID(set.ID) || (set.VersionID != nil && !validHookSnapshotID(set.VersionID)) || !validHookSnapshotText(set.Version, 0, false) {
			return invalid
		}
	}
	if target := c.Target; target != nil {
		if !validHookSnapshotID(target.ID) || !validHookSnapshotText(target.Type, 0, true) || (target.VersionID != nil && !validHookSnapshotID(target.VersionID)) {
			return invalid
		}
	}
	if phase == HookPhaseBefore {
		if c.TerminalStatus != nil || c.TerminalReason != nil {
			return invalid
		}
	} else if phase == HookPhaseAfter {
		switch c.GetTerminalStatus() {
		case spi.HookTerminalStatusSuccess, spi.HookTerminalStatusFailed, spi.HookTerminalStatusTerminated, spi.HookTerminalStatusSystemTerminated:
		default:
			return invalid
		}
		if !validHookSnapshotText(c.TerminalReason, 0, false) {
			return invalid
		}
	}
	if phase != HookPhaseBefore && phase != HookPhaseAfter {
		return invalid
	}
	return nil
}

func validHookSnapshotText(value *string, max int, required bool) bool {
	if value == nil {
		return !required
	}
	return utf8.ValidString(*value) && (!required || strings.TrimSpace(*value) != "") && (max == 0 || len(*value) <= max)
}

func validHookSnapshotID(value *string) bool {
	if value == nil || *value == "" {
		return false
	}
	for _, c := range *value {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, err := strconv.ParseInt(*value, 10, 64)
	return err == nil && n > 0
}
