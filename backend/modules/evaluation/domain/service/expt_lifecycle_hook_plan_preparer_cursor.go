// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type preparationCursor struct {
	Version      int                   `json:"v"`
	Key          entity.HookRunKey     `json:"key"`
	Fingerprint  string                `json:"fingerprint"`
	Phase        string                `json:"phase"`
	Route        string                `json:"route"`
	Selector     string                `json:"selector,omitempty"`
	Batch        int                   `json:"batch"`
	BatchDone    bool                  `json:"batch_done,omitempty"`
	Retries      int                   `json:"retries"`
	Selected     entity.HookPlanDigest `json:"selected"`
	Verified     entity.HookPlanDigest `json:"verified"`
	SourceID     int64                 `json:"source_id,omitempty"`
	SourceCount  int64                 `json:"source_count,omitempty"`
	SourceHash   string                `json:"source_hash,omitempty"`
	SourceDigest entity.HookPlanDigest `json:"source_digest"`
}

func readPreparationCursor(run *entity.HookStoredRun, fingerprint string) (preparationCursor, error) {
	root := entity.NewHookPlanDigest()
	c := preparationCursor{Version: 1, Key: run.State.Key, Fingerprint: fingerprint, Phase: "select", Selected: root, Verified: root, SourceDigest: root}
	if run.PlanCursor != "" {
		decoder := json.NewDecoder(strings.NewReader(run.PlanCursor))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&c); err != nil {
			return c, err
		}
		canonical, err := encodePreparationCursor(c)
		if err != nil || canonical != run.PlanCursor {
			return c, errors.New("invalid preparation cursor")
		}
	}
	if c.Version != 1 || c.Key != run.State.Key || c.Fingerprint != fingerprint || (c.Phase != "select" && c.Phase != "verify") || (c.Route != "" && c.Route != "normal" && c.Route != "copy") || c.Batch < 0 || c.Retries < 0 || c.Retries > 10 || c.Selected.Count != run.PlanCount || c.Verified.Count > c.Selected.Count {
		return c, errors.New("invalid preparation cursor")
	}
	for _, digest := range []entity.HookPlanDigest{c.Selected, c.Verified, c.SourceDigest} {
		if _, err := entity.AppendHookPlanDigest(digest, nil); err != nil {
			return c, err
		}
	}
	if c.Phase == "select" && c.Verified != root || run.Mode != entity.EvaluationModeRetryItems && (c.Batch != 0 || c.BatchDone) || c.BatchDone && c.Selector == "" || c.Route != "copy" && (c.SourceID != 0 || c.SourceCount != 0 || c.SourceHash != "" || c.SourceDigest != root) {
		return c, errors.New("invalid preparation phase")
	}
	if c.Route == "copy" && (run.Mode != entity.EvaluationModeFailRetry || c.SourceID <= 0 || c.SourceID == run.State.Key.RunID || run.SourceRunID == nil || *run.SourceRunID != c.SourceID || c.SourceCount < 0 || c.SourceDigest.Count > c.SourceCount) {
		return c, errors.New("invalid source cursor")
	}
	return c, nil
}

func encodePreparationCursor(c preparationCursor) (string, error) {
	if !utf8.ValidString(c.Selector) {
		return "", errors.New("invalid opaque selector cursor")
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(data) > 65535 {
		return "", errors.New("preparation cursor exceeds storage limit")
	}
	return string(data), nil
}

func validPreparationPage(page *entity.HookPlanReadPage, start int64, key entity.HookRunKey) bool {
	if start < 0 || page.Count < start || len(page.Items) > 100 || page.NextOrdinal != start+int64(len(page.Items)) || page.NextOrdinal > page.Count || page.HasMore != (page.NextOrdinal < page.Count) || page.HasMore && len(page.Items) == 0 {
		return false
	}
	if len(page.Items) > 0 {
		if (entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: key}, Items: page.Items}).Validate() != nil {
			return false
		}
	}
	return true
}

func preparationExperiment(raw *entity.Experiment) *entity.Experiment {
	e := &entity.Experiment{ID: raw.ID, SpaceID: raw.SpaceID, ExptType: raw.ExptType, EvalSetSourceType: raw.EvalSetSourceType, EvalSetID: raw.EvalSetID, EvalSetVersionID: raw.EvalSetVersionID, EvalSetSpaceID: raw.EvalSetSpaceID, TrialRunItemCount: raw.TrialRunItemCount}
	if raw.EvalSetID > 0 {
		space := resolveLoadSpaceID(raw.SpaceID, raw.EvalSetSpaceID)
		e.EvalSet = &entity.EvaluationSet{ID: raw.EvalSetID, SpaceID: space, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: raw.EvalSetVersionID, SpaceID: space, EvaluationSetID: raw.EvalSetID}}
	}
	if raw.EvalConf != nil {
		e.EvalConf = &entity.EvaluationConfiguration{}
		for _, sc := range raw.EvalConf.EvalSetConfigs {
			copy := &entity.EvalSetConfig{EvalSetID: sc.EvalSetID, EvalSetVersionID: sc.EvalSetVersionID, SourceSpaceID: sc.SourceSpaceID}
			if sc.ItemFilter != nil {
				filter := &entity.ExptItemFilter{QueryAndOr: sc.ItemFilter.QueryAndOr}
				for _, field := range sc.ItemFilter.FilterFields {
					if field == nil {
						filter.FilterFields = append(filter.FilterFields, nil)
						continue
					}
					f := *field
					f.Values = append([]string(nil), field.Values...)
					filter.FilterFields = append(filter.FilterFields, &f)
				}
				copy.ItemFilter = filter
			}
			e.EvalConf.EvalSetConfigs = append(e.EvalConf.EvalSetConfigs, copy)
		}
	}
	return e
}
