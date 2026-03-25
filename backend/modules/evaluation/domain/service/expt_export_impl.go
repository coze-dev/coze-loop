// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gopkg/util/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptResultExportService struct {
	txDB               db.Provider
	repo               repo.IExptResultExportRecordRepo
	exptRepo           repo.IExperimentRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptPublisher      events.ExptEventPublisher
	exptResultService  ExptResultService
	fileClient         fileserver.ObjectStorage
	configer           component.IConfiger
	benefitService     benefit.IBenefitService
	urlProcessor       component.IURLProcessor
	evalSetItemSvc     EvaluationSetItemService
}

func NewExptResultExportService(
	txDB db.Provider,
	repo repo.IExptResultExportRecordRepo,
	exptRepo repo.IExperimentRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptPublisher events.ExptEventPublisher,
	exptResultService ExptResultService,
	fileClient fileserver.ObjectStorage,
	configer component.IConfiger,
	benefitService benefit.IBenefitService,
	urlProcessor component.IURLProcessor,
	esis EvaluationSetItemService,
) IExptResultExportService {
	return &ExptResultExportService{
		repo:               repo,
		txDB:               txDB,
		exptTurnResultRepo: exptTurnResultRepo,
		exptPublisher:      exptPublisher,
		exptRepo:           exptRepo,
		exptResultService:  exptResultService,
		fileClient:         fileClient,
		configer:           configer,
		benefitService:     benefitService,
		urlProcessor:       urlProcessor,
		evalSetItemSvc:     esis,
	}
}

func (e ExptResultExportService) ExportCSV(ctx context.Context, spaceID, exptID int64, session *entity.Session) (int64, error) {
	// 检查实验是否完成
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return 0, err
	}
	if !entity.IsExptFinished(expt.Status) {
		return 0, errorx.NewByCode(errno.ExperimentUncompleteCode)
	}
	// 检查是否存在运行中的导出任务
	page := entity.NewPage(1, 1)
	_, total, err := e.repo.List(ctx, spaceID, exptID, page, ptr.Of(int32(entity.CSVExportStatus_Running)))
	if err != nil {
		return 0, err
	}
	const maxExportTaskNum = 3
	if total > maxExportTaskNum {
		return 0, errorx.NewByCode(errno.ExportRunningCountLimitCode)
	}

	if !e.configer.GetExptExportWhiteList(ctx).IsUserIDInWhiteList(session.UserID) {
		// 检查权益
		result, err := e.benefitService.BatchCheckEnableTypeBenefit(ctx, &benefit.BatchCheckEnableTypeBenefitParams{
			ConnectorUID:       session.UserID,
			SpaceID:            spaceID,
			EnableTypeBenefits: []string{"exp_download_report_enabled"},
		})
		if err != nil {
			return 0, err
		}

		if result == nil || result.Results == nil || !result.Results["exp_download_report_enabled"] {
			return 0, errorx.NewByCode(errno.ExperimentExportValidateFailCode)
		}
	}

	record := &entity.ExptResultExportRecord{
		SpaceID:         spaceID,
		ExptID:          exptID,
		CsvExportStatus: entity.CSVExportStatus_Running,
		CreatedBy:       session.UserID,
		StartAt:         gptr.Of(time.Now()),
	}
	exportID, err := e.repo.Create(ctx, record)
	if err != nil {
		return 0, err
	}

	exportEvent := &entity.ExportCSVEvent{
		ExportID:     exportID,
		ExperimentID: exptID,
		SpaceID:      spaceID,
		Session:      session,
	}
	err = e.exptPublisher.PublishExptExportCSVEvent(ctx, exportEvent, nil)
	if err != nil {
		return 0, err
	}

	return exportID, nil
}

func (e ExptResultExportService) GetExptExportRecord(ctx context.Context, spaceID, exportID int64) (*entity.ExptResultExportRecord, error) {
	exportRecord, err := e.repo.Get(ctx, spaceID, exportID)
	if err != nil {
		logger.CtxErrorf(ctx, "get export record error: %v", err)
		return nil, err
	}

	if exportRecord.FilePath != "" {
		var ttl int64 = 24 * 60 * 60
		signOpt := fileserver.SignWithTTL(time.Duration(ttl) * time.Second)

		signURL, _, err := e.fileClient.SignDownloadReq(ctx, exportRecord.FilePath, signOpt)
		if err != nil {
			return nil, err
		}
		signURL = e.urlProcessor.ProcessSignURL(ctx, signURL)
		exportRecord.URL = ptr.Of(signURL)
		logs.CtxInfo(ctx, "get export record sign url final: %v", signURL)
	}

	exportRecord.Expired = isExportRecordExpired(exportRecord.StartAt)

	return exportRecord, nil
}

func isExportRecordExpired(targetTime *time.Time) bool {
	if targetTime == nil {
		return false
	}
	now := time.Now()
	duration := now.Sub(*targetTime)
	oneHundredDays := 100 * 24 * time.Hour
	// 判断差值是否大于100天
	return duration > oneHundredDays
}

func (e ExptResultExportService) UpdateExportRecord(ctx context.Context, exportRecord *entity.ExptResultExportRecord) error {
	err := e.repo.Update(ctx, exportRecord)
	if err != nil {
		return err
	}

	return nil
}

func (e ExptResultExportService) ListExportRecord(ctx context.Context, spaceID, exptID int64, page entity.Page) ([]*entity.ExptResultExportRecord, int64, error) {
	records, total, err := e.repo.List(ctx, spaceID, exptID, page, nil)
	if err != nil {
		return nil, 0, err
	}

	for _, record := range records {
		record.Expired = isExportRecordExpired(record.StartAt)
	}

	return records, total, nil
}

func (e ExptResultExportService) HandleExportEvent(ctx context.Context, spaceID, exptID, exportID int64) (err error) {
	var fileName string
	defer func() {
		record := &entity.ExptResultExportRecord{
			ID:              exportID,
			SpaceID:         spaceID,
			ExptID:          exptID,
			CsvExportStatus: entity.CSVExportStatus_Success,
			FilePath:        fileName,
			EndAt:           gptr.Of(time.Now()),
		}

		if err != nil {
			errMsg := e.configer.GetErrCtrl(ctx).ConvertErrMsg(err.Error())
			logs.CtxWarn(ctx, "[DoExportCSV] store export err, before: %v, after: %v", err, errMsg)

			ei, ok := errno.ParseErrImpl(err)
			if !ok {
				clonedErr := errno.CloneErr(err)
				err = errno.NewTurnOtherErr(errMsg, clonedErr)
			} else {
				clonedErr := errno.CloneErr(err)
				err = ei.SetErrMsg(errMsg).SetCause(clonedErr)
			}

			record.CsvExportStatus = entity.CSVExportStatus_Failed
			record.ErrMsg = errno.SerializeErr(err)
		}

		err1 := e.repo.Update(ctx, record)
		if err1 != nil {
			if err == nil {
				err = err1
			}
		}
	}()

	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	fileName, err = e.getFileName(ctx, expt.Name, exportID)
	if err != nil {
		return err
	}

	err = e.DoExportCSV(ctx, spaceID, exptID, fileName, false)
	if err != nil {
		return err
	}

	return nil
}

func (e ExptResultExportService) DoExportCSV(ctx context.Context, spaceID, exptID int64, fileName string, withLogID bool) (err error) {
	const (
		pageSize = 20
		maxPage  = 2500
	)

	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err = file.WriteString("\xEF\xBB\xBF"); err != nil {
		return err
	}
	writer := csv.NewWriter(file)

	param := &entity.MGetExperimentResultParam{
		SpaceID:                   spaceID,
		ExptIDs:                   []int64{exptID},
		BaseExptID:                ptr.Of(exptID),
		LoadEvaluatorFullContent:  gptr.Of(false), // 导出 CSV 仅需 score/reason，不加载 Evaluator input/output 大对象
		LoadEvalTargetFullContent: gptr.Of(true),  // Target output 需完整内容（如 OutputFields 大字段）
		// 导出不包含 trajectory 列，且不需要完整轨迹 JSON，减轻 TOS/序列化压力
		FullTrajectory: false,
	}

	var helper *exportCSVHelper

	// 第一遍分页：收集所有 TargetOutput.OutputFields 中出现的键（排除 trajectory），避免自定义列仅出现在后页时漏列。
	extraOutputKeys := make(map[string]struct{})
	for pageNum := 1; pageNum <= maxPage; pageNum++ {
		param.Page = entity.NewPage(pageNum, pageSize)
		result, err := e.exptResultService.MGetExperimentResult(ctx, param)
		if err != nil {
			return err
		}
		collectEvalTargetOutputFieldKeysFromItemResults(result.ItemResults, extraOutputKeys)
		if pageNum*pageSize >= int(result.Total) {
			break
		}
	}

	for pageNum := 1; pageNum <= maxPage; pageNum++ {
		param.Page = entity.NewPage(pageNum, pageSize)
		result, err := e.exptResultService.MGetExperimentResult(ctx, param)
		if err != nil {
			return err
		}

		if pageNum == 1 {
			var colAnnotation []*entity.ColumnAnnotation
			for _, ca := range result.ExptColumnAnnotations {
				if ca.ExptID == exptID {
					colAnnotation = ca.ColumnAnnotations
					break
				}
			}
			schemaCols := filterTrajectoryEvalTargetColumns(pickExptColumnsEvalTargetColumns(result, exptID))
			columnsEvalTarget := mergeExtraEvalTargetOutputColumns(schemaCols, extraOutputKeys)
			helper = &exportCSVHelper{
				spaceID:            spaceID,
				exptID:             exptID,
				withLogID:          withLogID,
				colEvaluators:      result.ColumnEvaluators,
				colEvalSetFields:   result.ColumnEvalSetFields,
				colAnnotations:     colAnnotation,
				columnsEvalTarget:  columnsEvalTarget,
				exptRepo:           e.exptRepo,
				exptTurnResultRepo: e.exptTurnResultRepo,
				exptPublisher:      e.exptPublisher,
				exptResultService:  e.exptResultService,
				fileClient:         e.fileClient,
				evalSetItemSvc:     e.evalSetItemSvc,
			}
			columns, err := helper.buildColumns(ctx)
			if err != nil {
				return err
			}
			if err = writer.Write(columns); err != nil {
				return err
			}
		}

		rows, err := helper.buildRowsForItems(ctx, result.ItemResults)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err = writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}

		if pageNum*pageSize >= int(result.Total) {
			break
		}
	}

	if _, err = file.Seek(0, 0); err != nil {
		return err
	}
	if err = helper.uploadCSVFile(ctx, fileName, file); err != nil {
		return fmt.Errorf("uploadFile error: %v", err)
	}
	return os.Remove(fileName)
}

type exportCSVHelper struct {
	spaceID   int64
	exptID    int64
	withLogID bool

	colEvaluators     []*entity.ColumnEvaluator
	colEvalSetFields  []*entity.ColumnEvalSetField
	colAnnotations    []*entity.ColumnAnnotation
	allItemResults    []*entity.ItemResult
	columnsEvalTarget []*entity.ColumnEvalTarget

	exptRepo           repo.IExperimentRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptPublisher      events.ExptEventPublisher
	exptResultService  ExptResultService
	fileClient         fileserver.ObjectStorage
	evalSetItemSvc     EvaluationSetItemService
}

const (
	columnNameID            = "ID"
	columnNameStatus        = "status"
	columnNameLogID         = "logID"
	columnNameTargetTraceID = "targetTraceID"
	columnNameWeightedScore = "weightedScore"
)

// filterTrajectoryEvalTargetColumns 导出 CSV 不输出 trajectory 列，其余评测对象输出列（含 schema 自定义字段与性能指标）保留。
func filterTrajectoryEvalTargetColumns(cols []*entity.ColumnEvalTarget) []*entity.ColumnEvalTarget {
	if len(cols) == 0 {
		return cols
	}
	out := make([]*entity.ColumnEvalTarget, 0, len(cols))
	for _, col := range cols {
		if col == nil || col.Name == consts.ReportColumnNameEvalTargetTrajectory {
			continue
		}
		out = append(out, col)
	}
	return out
}

func pickExptColumnsEvalTargetColumns(report *entity.MGetExperimentReportResult, exptID int64) []*entity.ColumnEvalTarget {
	if report == nil {
		return nil
	}
	for _, row := range report.ExptColumnsEvalTarget {
		if row != nil && row.ExptID == exptID {
			return row.Columns
		}
	}
	if len(report.ExptColumnsEvalTarget) > 0 && report.ExptColumnsEvalTarget[0] != nil {
		return report.ExptColumnsEvalTarget[0].Columns
	}
	return nil
}

// collectEvalTargetOutputFieldKeysFromItemResults 汇总行数据里实际出现的 OutputFields 键（不含 trajectory），用于补齐 schema 未声明的自定义列。
func collectEvalTargetOutputFieldKeysFromItemResults(items []*entity.ItemResult, into map[string]struct{}) {
	if len(items) == 0 || into == nil {
		return
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		for _, turn := range item.TurnResults {
			if turn == nil || len(turn.ExperimentResults) == 0 || turn.ExperimentResults[0] == nil {
				continue
			}
			payload := turn.ExperimentResults[0].Payload
			if payload == nil || payload.TargetOutput == nil || payload.TargetOutput.EvalTargetRecord == nil {
				continue
			}
			data := payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData
			if data == nil || data.OutputFields == nil {
				continue
			}
			for k := range data.OutputFields {
				if k == "" || k == consts.EvalTargetOutputFieldKeyTrajectory {
					continue
				}
				into[k] = struct{}{}
			}
		}
	}
}

// mergeExtraEvalTargetOutputColumns 在 schema 列（已去 trajectory）之后追加数据中出现过、但 schema 未覆盖的输出字段列，保证自定义动态字段也能导出。
func mergeExtraEvalTargetOutputColumns(schema []*entity.ColumnEvalTarget, extraKeys map[string]struct{}) []*entity.ColumnEvalTarget {
	if len(extraKeys) == 0 {
		return schema
	}
	seen := make(map[string]struct{}, len(schema)+len(extraKeys))
	for _, c := range schema {
		if c != nil && c.Name != "" {
			seen[c.Name] = struct{}{}
		}
	}
	extras := make([]string, 0, len(extraKeys))
	for k := range extraKeys {
		if k == consts.EvalTargetOutputFieldKeyTrajectory {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		extras = append(extras, k)
	}
	if len(extras) == 0 {
		return schema
	}
	sort.Strings(extras)
	out := make([]*entity.ColumnEvalTarget, len(schema), len(schema)+len(extras))
	copy(out, schema)
	for _, k := range extras {
		out = append(out, &entity.ColumnEvalTarget{Name: k})
	}
	return out
}

func (e exportCSVHelper) buildColumns(ctx context.Context) ([]string, error) {
	columns := []string{}

	columns = append(columns, columnNameID, columnNameStatus)
	for _, colEvalSetField := range e.colEvalSetFields {
		if colEvalSetField == nil {
			continue
		}

		columns = append(columns, ptr.From(colEvalSetField.Name))
	}

	for _, col := range e.columnsEvalTarget {
		columns = append(columns, gcond.If(len(col.DisplayName) > 0, col.DisplayName, col.Name))
	}

	// colEvaluators
	for _, colEvaluator := range e.colEvaluators {
		if colEvaluator == nil {
			continue
		}

		columns = append(columns, getColumnNameEvaluator(ptr.From(colEvaluator.Name), ptr.From(colEvaluator.Version)))
		columns = append(columns, getColumnNameEvaluatorReason(ptr.From(colEvaluator.Name), ptr.From(colEvaluator.Version)))
	}

	// 加权得分列（如果有评估器，则添加加权得分列）
	if len(e.colEvaluators) > 0 {
		columns = append(columns, columnNameWeightedScore)
	}

	// colAnnotations
	for _, colAnnotation := range e.colAnnotations {
		if colAnnotation == nil {
			continue
		}

		columns = append(columns, colAnnotation.TagName)

	}

	// logID for analysis report
	if e.withLogID {
		columns = append(columns, columnNameLogID)
		columns = append(columns, columnNameTargetTraceID)
	}

	return columns, nil
}

func getColumnNameEvaluator(evaluatorName, version string) string {
	return fmt.Sprintf("%s<%s>", evaluatorName, version)
}

func getColumnNameEvaluatorReason(evaluatorName, version string) string {
	return fmt.Sprintf("%s<%s>_reason", evaluatorName, version)
}

func (e *exportCSVHelper) buildColumnEvalTargetContent(ctx context.Context, columnName string, data *entity.EvalTargetOutputData) (string, error) {
	if data == nil {
		return "", nil
	}
	switch columnName {
	case consts.ReportColumnNameEvalTargetTotalLatency:
		return strconv.FormatInt(gptr.Indirect(data.TimeConsumingMS), 10), nil
	case consts.ReportColumnNameEvalTargetInputTokens:
		return strconv.FormatInt(data.EvalTargetUsage.GetInputTokens(), 10), nil
	case consts.ReportColumnNameEvalTargetOutputTokens:
		return strconv.FormatInt(data.EvalTargetUsage.GetOutputTokens(), 10), nil
	case consts.ReportColumnNameEvalTargetTotalTokens:
		return strconv.FormatInt(data.EvalTargetUsage.GetTotalTokens(), 10), nil
	default:
		return e.toContentStr(ctx, data.OutputFields[columnName])
	}
}

func (e *exportCSVHelper) buildRows(ctx context.Context) ([][]string, error) {
	return e.buildRowsForItems(ctx, e.allItemResults)
}

func (e *exportCSVHelper) buildRowsForItems(ctx context.Context, itemResults []*entity.ItemResult) ([][]string, error) {
	rows := make([][]string, 0)
	for _, itemResult := range itemResults {
		if itemResult == nil {
			logs.CtxWarn(ctx, "itemResult is nil")
			continue
		}

		for _, turnResult := range itemResult.TurnResults {
			if turnResult == nil {
				logs.CtxWarn(ctx, "turnResult is nil")
				continue
			}

			rowData := make([]string, 0)
			rowData = append(rowData, strconv.Itoa(int(itemResult.ItemID)))
			runState := ""
			if itemResult.SystemInfo != nil {
				runState = itemRunStateToString(itemResult.SystemInfo.RunState)
			}
			rowData = append(rowData, runState)

			if len(turnResult.ExperimentResults) == 0 || turnResult.ExperimentResults[0] == nil {
				logs.CtxWarn(ctx, "turnResult.ExperimentResults is nil")
				continue
			}
			payload := turnResult.ExperimentResults[0].Payload
			if payload == nil ||
				payload.EvalSet == nil ||
				payload.EvalSet.Turn == nil ||
				payload.EvalSet.Turn.FieldDataList == nil {
				return nil, fmt.Errorf("FieldDataList is nil")
			}
			datasetFields, err := e.getDatasetFields(ctx, e.colEvalSetFields, payload.EvalSet)
			if err != nil {
				return nil, err
			}
			rowData = append(rowData, datasetFields...)

			for _, col := range e.columnsEvalTarget {
				if payload.TargetOutput != nil &&
					payload.TargetOutput.EvalTargetRecord != nil &&
					payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData != nil {
					cont, err := e.buildColumnEvalTargetContent(ctx, col.Name, payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData)
					if err != nil {
						return nil, err
					}
					rowData = append(rowData, cont)
				} else {
					rowData = append(rowData, "")
				}
			}

			// 评估器结果，按ColumnEvaluators的顺序排序
			evaluatorRecords := make(map[int64]*entity.EvaluatorRecord)
			if payload.EvaluatorOutput != nil &&
				payload.EvaluatorOutput.EvaluatorRecords != nil {
				evaluatorRecords = payload.EvaluatorOutput.EvaluatorRecords
			}

			for _, colEvaluator := range e.colEvaluators {
				if colEvaluator == nil {
					continue
				}

				evaluatorRecord := evaluatorRecords[colEvaluator.EvaluatorVersionID]
				rowData = append(rowData, getEvaluatorScore(evaluatorRecord))
				rowData = append(rowData, getEvaluatorReason(evaluatorRecord))
			}

			// 加权得分（如果有评估器，则添加加权得分数据）
			if len(e.colEvaluators) > 0 {
				weightedScore := ""
				if payload.EvaluatorOutput != nil && payload.EvaluatorOutput.WeightedScore != nil {
					weightedScore = strconv.FormatFloat(*payload.EvaluatorOutput.WeightedScore, 'f', 2, 64)
				}
				rowData = append(rowData, weightedScore)
			}

			// 标注结果，按Annotation的顺序排序
			if payload.AnnotateResult != nil && payload.AnnotateResult.AnnotateRecords != nil {
				annotateRecords := payload.AnnotateResult.AnnotateRecords
				for _, colAnnotation := range e.colAnnotations {
					if colAnnotation == nil {
						continue
					}

					annotateRecord := annotateRecords[colAnnotation.TagKeyID]
					rowData = append(rowData, getAnnotationData(annotateRecord, colAnnotation))
				}
			}

			// logID
			if e.withLogID {
				logID := ""
				if payload.SystemInfo != nil {
					logID = ptr.From(payload.SystemInfo.LogID)
				}
				traceID := ""
				if payload.TargetOutput != nil &&
					payload.TargetOutput.EvalTargetRecord != nil {
					traceID = payload.TargetOutput.EvalTargetRecord.TraceID
				}
				rowData = append(rowData, logID)
				rowData = append(rowData, traceID)

			}

			rows = append(rows, rowData)
		}
	}

	return rows, nil
}

func itemRunStateToString(itemRunState entity.ItemRunState) string {
	switch itemRunState {
	case entity.ItemRunState_Unknown:
		return "unknown"
	case entity.ItemRunState_Queueing:
		return "queueing"
	case entity.ItemRunState_Processing:
		return "processing"
	case entity.ItemRunState_Success:
		return "success"
	case entity.ItemRunState_Fail:
		return "fail"
	case entity.ItemRunState_Terminal:
		return "terminal"
	default:
		return ""
	}
}

// getDatasetFields 按顺序获取数据集字段
func (e *exportCSVHelper) getDatasetFields(ctx context.Context, colEvalSetFields []*entity.ColumnEvalSetField, tes *entity.TurnEvalSet) (fields []string, err error) {
	fdl := tes.Turn.FieldDataList
	fdm := slices.ToMap(fdl, func(t *entity.FieldData) (string, *entity.FieldData) { return t.Key, t })
	fields = make([]string, 0, len(colEvalSetFields))

	for _, colEvalSetField := range colEvalSetFields {
		if colEvalSetField == nil {
			continue
		}

		fieldData, ok := fdm[ptr.From(colEvalSetField.Key)]
		if !ok {
			fields = append(fields, "")
			continue
		}

		if fieldData.Content == nil {
			continue
		}

		if fieldData.Content.IsContentOmitted() {
			if fieldData, err = e.evalSetItemSvc.GetEvaluationSetItemField(ctx, &entity.GetEvaluationSetItemFieldParam{
				SpaceID:         e.spaceID,
				EvaluationSetID: tes.EvalSetID,
				ItemPK:          tes.ItemID,
				FieldName:       gptr.Indirect(colEvalSetField.Name),
				TurnID:          gptr.Of(tes.Turn.ID),
			}); err != nil {
				return nil, err
			}
		}

		data, err := e.toContentStr(ctx, fieldData.Content)
		if err != nil {
			return nil, err
		}

		fields = append(fields, data)
	}

	return fields, nil
}

func (e *exportCSVHelper) toContentStr(ctx context.Context, data *entity.Content) (string, error) {
	if data == nil {
		return "", nil
	}

	switch data.GetContentType() {
	case entity.ContentTypeText:
		return data.GetText(), nil
	case entity.ContentTypeImage, entity.ContentTypeAudio:
		return "", nil
	case entity.ContentTypeMultipart:
		return formatMultiPartData(data), nil
	default:
		return "", nil
	}
}

func formatMultiPartData(data *entity.Content) string {
	var builder strings.Builder
	for _, content := range data.MultiPart {
		switch content.GetContentType() {
		case entity.ContentTypeText:
			builder.WriteString(fmt.Sprintf("%s\n", content.GetText()))
		case entity.ContentTypeImage:
			url := ""
			if content.Image != nil && content.Image.URL != nil {
				url = fmt.Sprintf("<ref_image_url:%s>\n", *content.Image.URL)
			}
			builder.WriteString(url)
		case entity.ContentTypeAudio:
			url := ""
			if content.Audio != nil && content.Audio.URL != nil {
				url = fmt.Sprintf("<ref_audio_url:%s>\n", *content.Audio.URL)
			}
			builder.WriteString(url)
		case entity.ContentTypeVideo:
			url := ""
			if content.Video != nil && content.Video.URL != nil {
				url = fmt.Sprintf("<ref_video_url:%s>\n", *content.Video.URL)
			}
			builder.WriteString(url)
		case entity.ContentTypeMultipart:
			continue
		default:
			continue
		}
	}
	return builder.String()
}

func getEvaluatorScore(record *entity.EvaluatorRecord) string {
	if record == nil || record.EvaluatorOutputData == nil || record.EvaluatorOutputData.EvaluatorResult == nil || record.EvaluatorOutputData.EvaluatorResult.Score == nil {
		return ""
	}

	if record.EvaluatorOutputData.EvaluatorResult.Correction != nil {
		return strconv.FormatFloat(*record.EvaluatorOutputData.EvaluatorResult.Correction.Score, 'f', 2, 64) // 'f' 格式截取两位小数 {
	}

	return strconv.FormatFloat(*record.EvaluatorOutputData.EvaluatorResult.Score, 'f', 2, 64) // 'f' 格式截取两位小数)
}

func getEvaluatorReason(record *entity.EvaluatorRecord) string {
	if record == nil || record.EvaluatorOutputData == nil || record.EvaluatorOutputData.EvaluatorResult == nil {
		return ""
	}

	if record.EvaluatorOutputData.EvaluatorResult.Correction != nil {
		return record.EvaluatorOutputData.EvaluatorResult.Correction.Explain
	}

	return record.EvaluatorOutputData.EvaluatorResult.Reasoning
}

func getAnnotationData(record *entity.AnnotateRecord, columnAnnotation *entity.ColumnAnnotation) string {
	if record == nil || record.AnnotateData == nil {
		return ""
	}

	switch record.AnnotateData.TagContentType {
	case entity.TagContentTypeContinuousNumber:
		return strconv.FormatFloat(*record.AnnotateData.Score, 'f', 2, 64) // 'f' 格式截取两位小数)
	case entity.TagContentTypeCategorical, entity.TagContentTypeBoolean:
		for _, tagValue := range columnAnnotation.TagValues {
			if tagValue == nil {
				continue
			}
			if tagValue.TagValueId == record.TagValueID {
				return tagValue.TagValueName
			}
		}
		return ""
	case entity.TagContentTypeFreeText:
		return ptr.From(record.AnnotateData.TextValue)
	default:
		return ""
	}
}

func (e *ExptResultExportService) getFileName(ctx context.Context, exptName string, exportID int64) (string, error) {
	t := time.Now().Format("20060102")
	// 文件名为：{对应实验名}_实验报告_{导出任务ID}_{下载时间}.csv
	fileName := fmt.Sprintf("%s_实验报告_%d_%s.csv", exptName, exportID, t)
	return fileName, nil
}

func (e *exportCSVHelper) uploadCSVFile(ctx context.Context, fileName string, reader io.Reader) (err error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	logs.CtxDebug(ctx, "start upload, fileName: %s", fileName)
	if err = e.fileClient.Upload(ctx, fileName, reader); err != nil {
		logs.CtxError(ctx, "upload file failed, err: %v", err)
		return err
	}

	return nil
}
