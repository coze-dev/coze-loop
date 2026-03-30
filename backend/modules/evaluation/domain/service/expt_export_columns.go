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

// exportSpecMeansExportAll：请求未携带 export_columns（spec 为 nil）时导出全量列；一旦携带 spec（含空对象 {}）则按白名单，不再因「子字段全 nil」退回全量。
func exportSpecMeansExportAll(spec *entity.ExptResultExportColumnSpec) bool {
	return spec == nil
}

// mgetParamForExportSpec：spec 为 nil 时全量拉 Target；否则仅按显式列名（eval_target_outputs ∪ metrics）按需拉取；子字段 nil 与 [] 均表示该维度不导出、不触发全量拉取。
func mgetParamForExportSpec(spec *entity.ExptResultExportColumnSpec) *entity.MGetExperimentResultParam {
	p := &entity.MGetExperimentResultParam{
		LoadEvaluatorFullContent:  gptr.Of(false),
		LoadEvalTargetFullContent: gptr.Of(false),
		UseTurnListCursor:         true,
	}
	if exportSpecMeansExportAll(spec) {
		p.LoadEvalTargetFullContent = gptr.Of(true)
		p.FullTrajectory = true
		return p
	}
	var names []string
	if len(spec.EvalTargetOutputs) > 0 {
		names = append(names, spec.EvalTargetOutputs...)
	}
	if len(spec.Metrics) > 0 {
		names = append(names, spec.Metrics...)
	}
	names = dedupeStrings(names)
	if len(names) == 0 {
		p.FullTrajectory = false
		return p
	}
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
	if exportSpecMeansExportAll(spec) {
		return &exportColumnSelection{exportAll: true}
	}
	keys := make(map[string]struct{})
	keys[exportColKeyItemID] = struct{}{}
	keys[exportColKeyStatus] = struct{}{}

	// 评测集字段：仅非空列表为白名单；nil 与 [] 均不导出该组
	if len(spec.EvalSetFields) > 0 {
		for _, k := range dedupeStrings(spec.EvalSetFields) {
			keys[exportColPrefixEvalSet+k] = struct{}{}
		}
	}

	// Target 列白名单以请求名为准；不要求报告中 OutputSchema 已声明该列名（否则 schema 与请求不一致时只会导出评测集列）。
	// 报告中不存在的列仍导出表头，单元格按 buildColumnEvalTargetContent 从结构化字段 / OutputFields 取值，无数据则为空。
	// trajectory 与 mgetParamForExportSpec 中 FullTrajectory / LoadEvalTargetOutputFieldKeys 对齐，用户显式勾选时须导出该列。
	if len(spec.EvalTargetOutputs) > 0 {
		for _, name := range dedupeStrings(spec.EvalTargetOutputs) {
			keys[exportColPrefixTarget+name] = struct{}{}
		}
	}

	if len(spec.Metrics) > 0 {
		for _, name := range dedupeStrings(spec.Metrics) {
			keys[exportColPrefixTarget+name] = struct{}{}
		}
	}

	if len(spec.EvaluatorVersionIds) > 0 {
		for _, raw := range spec.EvaluatorVersionIds {
			addEvaluatorVersionIDKeysForExport(keys, raw)
		}
	}

	if spec.WeightedScore != nil && *spec.WeightedScore {
		keys[exportColKeyWeightedScore] = struct{}{}
	}

	// 白名单导出：不包含标注列（keys 中无 annotation:*，filterColumnAnnotationsForExport 得到空列表）

	return &exportColumnSelection{exportAll: false, keys: keys}
}

// addEvaluatorVersionIDKeysForExport 将 evaluator_version_ids 单条解析为评估器版本 ID（十进制整数字符串），并同时选中该版本的分数列与原因列。
func addEvaluatorVersionIDKeysForExport(keys map[string]struct{}, raw string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return
	}
	vid, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return
	}
	keys[evaluatorColumnToken(vid, "score")] = struct{}{}
	keys[evaluatorColumnToken(vid, "reason")] = struct{}{}
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

// ensureTargetColumnsForExportWhitelist 在报告列元数据之后，为白名单请求但报告中未出现的 Target 列补一条仅含 Name 的 ColumnEvalTarget，保证 CSV 表头与行循环能覆盖用户显式请求的列名。
func ensureTargetColumnsForExportWhitelist(
	spec *entity.ExptResultExportColumnSpec,
	filtered []*entity.ColumnEvalTarget,
	sel *exportColumnSelection,
) []*entity.ColumnEvalTarget {
	if sel == nil || sel.exportAll || spec == nil {
		return filtered
	}
	seen := make(map[string]struct{}, len(filtered)+8)
	out := make([]*entity.ColumnEvalTarget, 0, len(filtered)+8)
	for _, c := range filtered {
		if c == nil || c.Name == "" {
			continue
		}
		if !sel.includeTargetColumnName(c.Name) {
			continue
		}
		seen[c.Name] = struct{}{}
		out = append(out, c)
	}
	for _, name := range dedupeStrings(spec.EvalTargetOutputs) {
		if _, ok := seen[name]; ok {
			continue
		}
		if !sel.includeTargetColumnName(name) {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, &entity.ColumnEvalTarget{Name: name})
	}
	for _, name := range dedupeStrings(spec.Metrics) {
		if _, ok := seen[name]; ok {
			continue
		}
		if !sel.includeTargetColumnName(name) {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, &entity.ColumnEvalTarget{Name: name})
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
