// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// 内部扁平列键（CSV 构建与 TOS 按需加载共用）。
const (
	exportColKeyItemID        = "item_id"
	exportColKeyStatus        = "status"
	exportColPrefixEvalSet    = "eval_set:"
	exportColPrefixTarget     = "target:"
	exportColPrefixEvaluator  = "evaluator:"
	exportColKeyWeightedScore = "weighted_score"
	exportColPrefixAnnotation = "annotation:"
)

var exportColTargetMetricNames = map[string]struct{}{
	consts.ReportColumnNameEvalTargetTotalLatency: {},
	consts.ReportColumnNameEvalTargetInputTokens:  {},
	consts.ReportColumnNameEvalTargetOutputTokens: {},
	consts.ReportColumnNameEvalTargetTotalTokens:  {},
}

type exportColumnSelection struct {
	exportAll bool
	keys      map[string]struct{}
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func splitReportTargetNamesByMetric(cols []*entity.ColumnEvalTarget) (nonMetric, metric map[string]struct{}) {
	nonMetric = make(map[string]struct{})
	metric = make(map[string]struct{})
	for _, c := range cols {
		if c == nil || c.Name == "" {
			continue
		}
		if _, isM := exportColTargetMetricNames[c.Name]; isM {
			metric[c.Name] = struct{}{}
		} else {
			nonMetric[c.Name] = struct{}{}
		}
	}
	return nonMetric, metric
}

// exportSpecMeansExportAll：未带 export_columns，或四个一级 list 均未出现（{}）。
func exportSpecMeansExportAll(spec *entity.ExptResultExportColumnSpec) bool {
	if spec == nil {
		return true
	}
	return spec.EvalSetFields == nil && spec.EvalTargetOutputs == nil && spec.Metrics == nil &&
		spec.EvaluatorVersionIds == nil
}

// mgetParamForExportSpec：eval_target_outputs 未传时无法确定自定义列，保守全量拉 Target；仅依据「评测对象输出」列名决定 TOS（性能指标不占 TOS）。
func mgetParamForExportSpec(spec *entity.ExptResultExportColumnSpec) *entity.MGetExperimentResultParam {
	p := &entity.MGetExperimentResultParam{
		LoadEvaluatorFullContent:  gptr.Of(false),
		LoadEvalTargetFullContent: gptr.Of(false),
		UseTurnListCursor:         true,
	}
	if spec == nil || exportSpecMeansExportAll(spec) {
		p.LoadEvalTargetFullContent = gptr.Of(true)
		p.FullTrajectory = true
		return p
	}
	if spec.EvalTargetOutputs == nil {
		p.LoadEvalTargetFullContent = gptr.Of(true)
		p.FullTrajectory = true
		return p
	}
	if len(spec.EvalTargetOutputs) == 0 {
		p.FullTrajectory = false
		return p
	}
	names := dedupeStrings(spec.EvalTargetOutputs)
	var loadKeys []string
	hasTraj := false
	for _, name := range names {
		if name == consts.ReportColumnNameEvalTargetTrajectory {
			hasTraj = true
		}
		if _, isMetric := exportColTargetMetricNames[name]; isMetric {
			continue
		}
		loadKeys = append(loadKeys, name)
	}
	loadKeys = dedupeStrings(loadKeys)
	if len(loadKeys) > 0 {
		p.LoadEvalTargetOutputFieldKeys = loadKeys
	}
	p.FullTrajectory = hasTraj
	return p
}

func newExportColumnSelectionFromSpec(
	spec *entity.ExptResultExportColumnSpec,
	report *entity.MGetExperimentReportResult,
	exptID int64,
) *exportColumnSelection {
	if spec == nil || exportSpecMeansExportAll(spec) {
		return &exportColumnSelection{exportAll: true}
	}
	keys := make(map[string]struct{})
	keys[exportColKeyItemID] = struct{}{}
	keys[exportColKeyStatus] = struct{}{}

	// 评测集字段
	if spec.EvalSetFields == nil {
		for _, f := range report.ColumnEvalSetFields {
			if f != nil && f.Key != nil {
				keys[exportColPrefixEvalSet+ptr.From(f.Key)] = struct{}{}
			}
		}
	} else {
		for _, k := range dedupeStrings(spec.EvalSetFields) {
			keys[exportColPrefixEvalSet+k] = struct{}{}
		}
	}

	targetCols := pickEvalTargetColsForExpt(report, exptID)
	nonMetricInReport, metricInReport := splitReportTargetNamesByMetric(targetCols)

	// 评测对象输出（非性能指标）
	if spec.EvalTargetOutputs == nil {
		for name := range nonMetricInReport {
			keys[exportColPrefixTarget+name] = struct{}{}
		}
	} else if len(spec.EvalTargetOutputs) > 0 {
		for _, name := range dedupeStrings(spec.EvalTargetOutputs) {
			if _, ok := nonMetricInReport[name]; ok {
				keys[exportColPrefixTarget+name] = struct{}{}
			}
		}
	}

	// 性能指标
	if spec.Metrics == nil {
		for name := range metricInReport {
			keys[exportColPrefixTarget+name] = struct{}{}
		}
	} else if len(spec.Metrics) > 0 {
		for _, name := range dedupeStrings(spec.Metrics) {
			if _, ok := metricInReport[name]; ok {
				keys[exportColPrefixTarget+name] = struct{}{}
			}
		}
	}

	// 评估器版本列选择
	if spec.EvaluatorVersionIds == nil {
		for _, ev := range report.ColumnEvaluators {
			if ev == nil {
				continue
			}
			keys[evaluatorColumnToken(ev.EvaluatorVersionID, "score")] = struct{}{}
			keys[evaluatorColumnToken(ev.EvaluatorVersionID, "reason")] = struct{}{}
		}
		if len(report.ColumnEvaluators) > 0 {
			keys[exportColKeyWeightedScore] = struct{}{}
		}
	} else if len(spec.EvaluatorVersionIds) > 0 {
		for _, raw := range spec.EvaluatorVersionIds {
			addEvaluatorOutputToken(keys, strings.TrimSpace(raw))
		}
	}

	// 部分导出：不包含标注列（keys 中无 annotation:*，filterColumnAnnotationsForExport 得到空列表）

	return &exportColumnSelection{exportAll: false, keys: keys}
}

// addEvaluatorOutputToken 解析 evaluator_version_ids 单条 token。
func addEvaluatorOutputToken(keys map[string]struct{}, s string) {
	if s == "" {
		return
	}
	if s == exportColKeyWeightedScore || strings.EqualFold(s, "weighted_score") {
		keys[exportColKeyWeightedScore] = struct{}{}
		return
	}
	if strings.HasPrefix(s, exportColPrefixEvaluator) {
		rest := strings.TrimPrefix(s, exportColPrefixEvaluator)
		parts := strings.Split(rest, ":")
		if len(parts) >= 2 {
			vid, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			if err != nil {
				return
			}
			part := strings.ToLower(strings.TrimSpace(parts[1]))
			if part == "score" {
				keys[evaluatorColumnToken(vid, "score")] = struct{}{}
			}
			if part == "reason" {
				keys[evaluatorColumnToken(vid, "reason")] = struct{}{}
			}
		}
		return
	}
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return
	}
	part := strings.ToLower(strings.TrimSpace(s[i+1:]))
	if part != "score" && part != "reason" {
		return
	}
	vid, err := strconv.ParseInt(strings.TrimSpace(s[:i]), 10, 64)
	if err != nil {
		return
	}
	keys[evaluatorColumnToken(vid, part)] = struct{}{}
}

func pickEvalTargetColsForExpt(report *entity.MGetExperimentReportResult, exptID int64) []*entity.ColumnEvalTarget {
	if report == nil {
		return nil
	}
	for _, ect := range report.ExptColumnsEvalTarget {
		if ect != nil && ect.ExptID == exptID {
			return ect.Columns
		}
	}
	if len(report.ExptColumnsEvalTarget) > 0 && report.ExptColumnsEvalTarget[0] != nil {
		return report.ExptColumnsEvalTarget[0].Columns
	}
	return nil
}

func (s *exportColumnSelection) mgetExperimentResultParam(spaceID, exptID int64) *entity.MGetExperimentResultParam {
	p := &entity.MGetExperimentResultParam{
		SpaceID:                   spaceID,
		ExptIDs:                   []int64{exptID},
		BaseExptID:                ptr.Of(exptID),
		LoadEvaluatorFullContent:  gptr.Of(false),
		LoadEvalTargetFullContent: gptr.Of(false),
	}
	if s == nil || s.exportAll {
		p.LoadEvalTargetFullContent = gptr.Of(true)
		p.FullTrajectory = true
		return p
	}
	loadKeys := s.evalTargetOutputFieldKeysForLoad()
	if len(loadKeys) > 0 {
		p.LoadEvalTargetOutputFieldKeys = loadKeys
	}
	p.FullTrajectory = s.hasTargetColumn(consts.ReportColumnNameEvalTargetTrajectory)
	return p
}

func (s *exportColumnSelection) hasTargetColumn(name string) bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[exportColPrefixTarget+name]
	return ok
}

func (s *exportColumnSelection) evalTargetOutputFieldKeysForLoad() []string {
	if s == nil || s.exportAll {
		return nil
	}
	seen := make(map[string]struct{})
	var keys []string
	for k := range s.keys {
		if !strings.HasPrefix(k, exportColPrefixTarget) {
			continue
		}
		name := strings.TrimPrefix(k, exportColPrefixTarget)
		if name == "" {
			continue
		}
		if _, isMetric := exportColTargetMetricNames[name]; isMetric {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		keys = append(keys, name)
	}
	return keys
}

func (s *exportColumnSelection) includeEvalSetFieldKey(fieldKey string) bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[exportColPrefixEvalSet+fieldKey]
	return ok
}

func (s *exportColumnSelection) includeTargetColumnName(name string) bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[exportColPrefixTarget+name]
	return ok
}

func (s *exportColumnSelection) includeEvaluatorScore(versionID int64) bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[evaluatorColumnToken(versionID, "score")]
	return ok
}

func (s *exportColumnSelection) includeEvaluatorReason(versionID int64) bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[evaluatorColumnToken(versionID, "reason")]
	return ok
}

func (s *exportColumnSelection) includeWeightedScore() bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[exportColKeyWeightedScore]
	return ok
}

func (s *exportColumnSelection) includeAnnotationTag(tagKeyID int64) bool {
	if s == nil || s.exportAll {
		return true
	}
	_, ok := s.keys[exportColPrefixAnnotation+strconv.FormatInt(tagKeyID, 10)]
	return ok
}

func evaluatorColumnToken(versionID int64, part string) string {
	return fmt.Sprintf("%s%d:%s", exportColPrefixEvaluator, versionID, part)
}

func filterColumnEvalSetFieldsForExport(fields []*entity.ColumnEvalSetField, sel *exportColumnSelection) []*entity.ColumnEvalSetField {
	if sel == nil || sel.exportAll {
		return fields
	}
	out := make([]*entity.ColumnEvalSetField, 0, len(fields))
	for _, f := range fields {
		if f == nil || f.Key == nil {
			continue
		}
		if sel.includeEvalSetFieldKey(ptr.From(f.Key)) {
			out = append(out, f)
		}
	}
	return out
}

func filterColumnsEvalTargetForExport(cols []*entity.ColumnEvalTarget, sel *exportColumnSelection) []*entity.ColumnEvalTarget {
	if sel == nil || sel.exportAll {
		return cols
	}
	out := make([]*entity.ColumnEvalTarget, 0, len(cols))
	for _, c := range cols {
		if c == nil {
			continue
		}
		if sel.includeTargetColumnName(c.Name) {
			out = append(out, c)
		}
	}
	return out
}

func filterColumnEvaluatorsForExport(evs []*entity.ColumnEvaluator, sel *exportColumnSelection) []*entity.ColumnEvaluator {
	if sel == nil || sel.exportAll {
		return evs
	}
	out := make([]*entity.ColumnEvaluator, 0, len(evs))
	for _, ev := range evs {
		if ev == nil {
			continue
		}
		if sel.includeEvaluatorScore(ev.EvaluatorVersionID) || sel.includeEvaluatorReason(ev.EvaluatorVersionID) {
			out = append(out, ev)
		}
	}
	return out
}

func filterColumnAnnotationsForExport(ann []*entity.ColumnAnnotation, sel *exportColumnSelection) []*entity.ColumnAnnotation {
	if sel == nil || sel.exportAll {
		return ann
	}
	out := make([]*entity.ColumnAnnotation, 0, len(ann))
	for _, a := range ann {
		if a == nil {
			continue
		}
		if sel.includeAnnotationTag(a.TagKeyID) {
			out = append(out, a)
		}
	}
	return out
}
