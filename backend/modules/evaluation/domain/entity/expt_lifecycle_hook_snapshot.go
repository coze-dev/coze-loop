// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

// Inputs must come from the authorized Manager/user adapter, not request parameters.
type HookRunSnapshotInput struct {
	Key            HookRunKey
	ExecutionScope string
	CreatedAt      time.Time
	Config         *LifecycleHookConf
	Context        *spi.HookRunContext
	Selection      *HookSelectionSeed
	Execution      *HookExecutionSnapshot
	Schedule       *HookScheduleSeed
}

// HookRunSnapshot owns its frozen data; accessors return independent copies.
type HookRunSnapshot struct{ input HookRunSnapshotInput }

func NewHookRunSnapshot(in HookRunSnapshotInput) (*HookRunSnapshot, error) {
	invalid := errors.New("invalid frozen hook snapshot")
	if !validHookRunKey(in.Key) || !hookStorageASCII(in.ExecutionScope, 128) || in.CreatedAt.IsZero() || in.CreatedAt.Year() < 1 || in.CreatedAt.Year() > 9999 {
		return nil, invalid
	}
	if err := ValidateHookRunContext(in.Context, HookPhaseBefore); err != nil {
		return nil, invalid
	}
	if in.Context.GetWorkspaceID() != strconv.FormatInt(in.Key.WorkspaceID, 10) || in.Context.GetExperimentID() != strconv.FormatInt(in.Key.ExperimentID, 10) || in.Context.GetRunID() != strconv.FormatInt(in.Key.RunID, 10) {
		return nil, invalid
	}
	config, err := ResolveLifecycleHookConf(nil, in.Config)
	if err != nil || config == nil || (!snapshotStageEnabled(config.Before) && !snapshotStageEnabled(config.After)) {
		return nil, invalid
	}
	in.Config = config
	if in.Selection != nil {
		if in.Selection.Validate() != nil {
			return nil, invalid
		}
		in.Selection = copyHookValue(in.Selection)
	}
	if err := validateHookExecutionSnapshot(in); err != nil {
		return nil, invalid
	}
	in.Execution, err = cloneHookExecutionSnapshot(in.Execution)
	if err != nil {
		return nil, invalid
	}
	if in.Schedule != nil {
		mode := EvaluationModeSubmit
		switch in.Context.GetRunMode() {
		case "submit":
		case "trial_run":
			mode = EvaluationModeTrialRun
		case "fail_retry":
			mode = EvaluationModeFailRetry
		case "retry_all":
			mode = EvaluationModeRetryAll
		case "retry_items":
			mode = EvaluationModeRetryItems
		case "append":
			mode = EvaluationModeAppend
		default:
			return nil, invalid
		}
		if mode == EvaluationModeAppend && in.Context.GetExperiment().GetType() != "online" || mode != EvaluationModeAppend && in.Context.GetExperiment().GetType() != "offline" {
			return nil, invalid
		}
		if _, err := in.Schedule.Event(in.Key, in.ExecutionScope, mode, in.Context.GetInitiator().GetUserID()); err != nil {
			return nil, invalid
		}
		in.Schedule = cloneHookScheduleSeed(in.Schedule)
	}
	in.Context = cloneHookSnapshotContext(in.Context)
	in.CreatedAt = in.CreatedAt.UTC().Truncate(time.Millisecond)
	return &HookRunSnapshot{input: in}, nil
}

func (s *HookRunSnapshot) Input() HookRunSnapshotInput {
	if s == nil {
		return HookRunSnapshotInput{}
	}
	out := s.input
	out.Config, _ = ResolveLifecycleHookConf(nil, s.input.Config)
	out.Context = cloneHookSnapshotContext(s.input.Context)
	out.Selection = copyHookValue(s.input.Selection)
	out.Execution, _ = cloneHookExecutionSnapshot(s.input.Execution)
	out.Schedule = cloneHookScheduleSeed(s.input.Schedule)
	return out
}

func snapshotStageEnabled(c *HookConfig) bool { return c != nil && c.Enabled != nil && *c.Enabled }

func cloneHookSnapshotContext(c *spi.HookRunContext) *spi.HookRunContext {
	if c == nil {
		return nil
	}
	out := *c
	out.WorkspaceID = copyHookValue(c.WorkspaceID)
	out.ExperimentID = copyHookValue(c.ExperimentID)
	out.RunID = copyHookValue(c.RunID)
	out.RunMode = copyHookValue(c.RunMode)
	out.TerminalStatus = copyHookValue(c.TerminalStatus)
	out.TerminalReason = copyHookValue(c.TerminalReason)
	if c.Initiator != nil {
		u := *c.Initiator
		u.UserID = copyHookValue(u.UserID)
		u.IdentityType = copyHookValue(u.IdentityType)
		u.Email = copyHookValue(u.Email)
		u.Name = copyHookValue(u.Name)
		out.Initiator = &u
	}
	if c.Experiment != nil {
		e := *c.Experiment
		e.Name = copyHookValue(e.Name)
		e.Type = copyHookValue(e.Type)
		out.Experiment = &e
	}
	if c.EvalSets != nil {
		out.EvalSets = make([]*spi.HookEvalSetRef, len(c.EvalSets))
		for i, set := range c.EvalSets {
			if set != nil {
				v := *set
				v.WorkspaceID = copyHookValue(v.WorkspaceID)
				v.ID = copyHookValue(v.ID)
				v.VersionID = copyHookValue(v.VersionID)
				v.Version = copyHookValue(v.Version)
				out.EvalSets[i] = &v
			}
		}
	}
	if c.Target != nil {
		v := *c.Target
		v.ID = copyHookValue(v.ID)
		v.VersionID = copyHookValue(v.VersionID)
		v.Type = copyHookValue(v.Type)
		out.Target = &v
	}
	return &out
}
