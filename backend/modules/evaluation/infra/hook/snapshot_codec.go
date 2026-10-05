// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Override generated omitempty so an empty Online collection survives storage.
type storedHookContext struct {
	*spi.HookRunContext
	EvalSets []*spi.HookEvalSetRef `json:"eval_sets"`
}
type snapshotPayload struct {
	Version        int                           `json:"version"`
	Purpose        string                        `json:"purpose"`
	Key            entity.HookRunKey             `json:"run_key"`
	ExecutionScope string                        `json:"execution_scope"`
	CreatedAt      time.Time                     `json:"created_at"`
	Config         *entity.LifecycleHookConf     `json:"config"`
	Context        *storedHookContext            `json:"context"`
	Selection      *entity.HookSelectionSeed     `json:"selection,omitempty"`
	Execution      *entity.HookExecutionSnapshot `json:"execution,omitempty"`
	Schedule       *entity.HookScheduleSeed      `json:"schedule,omitempty"`
}

func (c *StorageCodec) EncodeSnapshot(ctx context.Context, key string, s *entity.HookRunSnapshot) (entity.HookProtectedSnapshot, error) {
	empty := entity.HookProtectedSnapshot{}
	// Validate zero values as well as constructor-created snapshots.
	validated, err := entity.NewHookRunSnapshot(s.Input())
	if err != nil {
		return empty, errHookStorageCodec
	}
	in := validated.Input()
	if !snapshotBusinessContentFits(in) {
		return empty, errHookStorageCodec
	}
	payload := snapshotPayload{Version: 1, Purpose: "run_snapshot", Key: in.Key, ExecutionScope: in.ExecutionScope, CreatedAt: in.CreatedAt, Config: in.Config, Context: &storedHookContext{HookRunContext: in.Context, EvalSets: in.Context.EvalSets}, Selection: in.Selection, Execution: in.Execution, Schedule: in.Schedule}
	plain, err := json.Marshal(payload)
	if err != nil {
		return empty, errHookStorageCodec
	}
	encrypted, hash, err := c.protect(ctx, key, plain)
	if err != nil {
		return empty, err
	}
	return entity.HookProtectedSnapshot{Cipher: encrypted, KeyID: key, Hash: hash, ExecutionScope: in.ExecutionScope}, nil
}

func (c *StorageCodec) DecodeSnapshot(ctx context.Context, key entity.HookRunKey, scope string, p entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
	if key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID <= 0 || !hookReference(scope, 128) || p.ExecutionScope != scope {
		return nil, errHookStorageCodec
	}
	plain, err := c.unprotect(ctx, p.KeyID, p.Cipher, p.Hash)
	if err != nil {
		return nil, err
	}
	var payload snapshotPayload
	if decodeHookStorageJSON(plain, &payload) != nil || payload.Version != 1 || payload.Purpose != "run_snapshot" || payload.Key != key || payload.ExecutionScope != scope || payload.Context == nil || payload.Context.HookRunContext == nil {
		return nil, errHookStorageCodec
	}
	payload.Context.HookRunContext.EvalSets = payload.Context.EvalSets
	s, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: payload.Key, ExecutionScope: payload.ExecutionScope, CreatedAt: payload.CreatedAt, Config: payload.Config, Context: payload.Context.HookRunContext, Selection: payload.Selection, Execution: payload.Execution, Schedule: payload.Schedule})
	if err != nil || !snapshotBusinessContentFits(s.Input()) {
		return nil, errHookStorageCodec
	}
	return s, nil
}

// This is a lower-bound check before operation IDs exist. BuildRequest checks
// the complete 64 KiB body using the actual operation and claim before sending.
func snapshotBusinessContentFits(in entity.HookRunSnapshotInput) bool {
	if in.Context == nil || in.Config == nil {
		return false
	}
	for _, config := range []*entity.HookConfig{in.Config.Before, in.Config.After} {
		if config == nil || config.Enabled == nil || !*config.Enabled {
			continue
		}
		payload := struct {
			Context    storedHookContext `json:"context"`
			Parameters json.RawMessage   `json:"parameters"`
		}{storedHookContext{HookRunContext: in.Context, EvalSets: in.Context.EvalSets}, json.RawMessage(*config.ParametersJSON)}
		body, err := json.Marshal(payload)
		if err != nil || len(body) > 64*1024 {
			return false
		}
	}
	return true
}

// DecodePhase supplies ClaimAttempt with a policy and the hash of the actual
// decrypted payload. It grants no permission to claim or send the operation.
func (c *StorageCodec) DecodePhase(ctx context.Context, key entity.HookRunKey, scope string, p entity.HookProtectedSnapshot, phase entity.HookPhase) (*entity.HookConfig, string, error) {
	s, err := c.DecodeSnapshot(ctx, key, scope, p)
	if err != nil {
		return nil, "", err
	}
	conf := s.Input().Config
	var selected *entity.HookConfig
	switch phase {
	case entity.HookPhaseBefore:
		selected = conf.Before
	case entity.HookPhaseAfter:
		selected = conf.After
	default:
		return nil, "", errHookStorageCodec
	}
	if selected == nil || selected.Enabled == nil || !*selected.Enabled {
		return nil, "", errHookStorageCodec
	}
	return selected, p.Hash, nil
}
