// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// HookSelectionSeed is private recovery metadata, never SPI context or parameters.
type HookSelectionSeed struct {
	Version            int    `json:"version"`
	TrialRunItemCount  int64  `json:"trial_run_item_count"`
	HasExplicitItemIDs bool   `json:"has_explicit_item_ids"`
	ConfigFingerprint  string `json:"config_fingerprint"`
}

func (s *HookSelectionSeed) Validate() error {
	if s == nil || s.Version != 1 || len(s.ConfigFingerprint) != sha256.Size*2 || s.HasExplicitItemIDs && s.TrialRunItemCount <= 0 {
		return errors.New("invalid hook selection seed")
	}
	for _, c := range s.ConfigFingerprint {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return errors.New("invalid hook selection fingerprint")
		}
	}
	return nil
}

// Only selection inputs are hashed; runtime concurrency and unrelated metadata may change.
func HookSelectionConfigFingerprint(expt *Experiment) (string, error) {
	if expt == nil {
		return "", errors.New("missing hook selection configuration")
	}
	type setConfig struct {
		ID            int64           `json:"id"`
		VersionID     int64           `json:"version_id"`
		SourceSpaceID int64           `json:"source_space_id"`
		ItemFilter    *ExptItemFilter `json:"item_filter"`
	}
	input := struct {
		SourceType        ExptEvalSetSourceType `json:"source_type"`
		Main              setConfig             `json:"main"`
		TrialRunItemCount int64                 `json:"trial_run_item_count"`
		Sets              []setConfig           `json:"sets"`
	}{SourceType: expt.EvalSetSourceType, Main: setConfig{ID: expt.EvalSetID, VersionID: expt.EvalSetVersionID, SourceSpaceID: expt.EvalSetSpaceID}, TrialRunItemCount: expt.TrialRunItemCount, Sets: make([]setConfig, 0)}
	if expt.EvalConf != nil {
		for _, set := range expt.EvalConf.EvalSetConfigs {
			if set == nil {
				return "", errors.New("invalid hook selection collection")
			}
			input.Sets = append(input.Sets, setConfig{ID: set.EvalSetID, VersionID: set.EvalSetVersionID, SourceSpaceID: set.SourceSpaceID, ItemFilter: set.ItemFilter})
		}
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
