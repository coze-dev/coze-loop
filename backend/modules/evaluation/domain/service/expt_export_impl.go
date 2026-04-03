// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
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
	"github.com/coze-dev/coze-loop/backend/pkg/json"
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

func (e ExptResultExportService) ExportCSV(ctx context.Context, spaceID, exptID int64, session *entity.Session, exportColumnSpec *entity.ExptResultExportColumnSpec) (int64, error) {
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
		ExportID:      exportID,
		ExperimentID:  exptID,
		SpaceID:       spaceID,
		Session:       session,
		ExportColumns: cloneExptExportColumnSpec(exportColumnSpec),
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

func (e ExptResultExportService) HandleExportEvent(ctx context.Context, event *entity.ExportCSVEvent) (err error) {
	if event == nil {
		return fmt.Errorf("export csv event is nil")
	}
	spaceID, exptID, exportID := event.SpaceID, event.ExperimentID, event.ExportID
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

	err = e.DoExportCSV(ctx, spaceID, exptID, fileName, false, event.ExportColumns)
	if err != nil {
		return err
	}

	return nil
}

func (e ExptResultExportService) DoExportCSV(ctx context.Context, spaceID, exptID int64, fileName string, withLogID bool, exportColumnSpec *entity.ExptResultExportColumnSpec) (err error) {
	const (
		pageSize   = 20
		maxBatches = 50000 // 游标分页安全上限，避免异常任务死循环
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

	param := mgetParamForExportSpec(exportColumnSpec)
	param.SpaceID = spaceID
	param.ExptIDs = []int64{exptID}
	param.BaseExptID = ptr.Of(exptID)
	param.LoadEvaluatorFullContent = gptr.Of(false) // 导出 CSV 仅需 score/reason，不加载 Evaluator input 大对象

	var helper *exportCSVHelper
	var sel *exportColumnSelection
	var turnCursor *entity.ExptTurnResultListCursor

	for batch := 0; batch < maxBatches; batch++ {
		param.Page = entity.NewPage(1, pageSize)
		param.TurnListCursor = turnCursor
		result, err := e.exptResultService.MGetExperimentResult(ctx, param)
		if err != nil {
			return err
		}

		if batch == 0 {
			sel = newExportColumnSelectionFromSpec(exportColumnSpec, result, exptID)
			var colAnnotation []*entity.ColumnAnnotation
			for _, ca := range result.ExptColumnAnnotations {
				if ca.ExptID == exptID {
					colAnnotation = ca.ColumnAnnotations
					break
				}
			}
			// 必须与 newExportColumnSelectionFromSpec 使用同一套 Target 列定义（按 exptID），不能用 ExptColumnsEvalTarget[0]，
			// 否则对比实验等场景下列顺序/实验不一致时，白名单里有 actual_output/metrics 但此处列元数据来自错误实验，filter 后只剩评测集列。
			baseTargetCols := pickEvalTargetColsForExpt(result, exptID)
			targetColsFiltered := filterColumnsEvalTargetForExport(baseTargetCols, sel)
			columnsEvalTarget := ensureTargetColumnsForExportWhitelist(exportColumnSpec, targetColsFiltered, sel)
			helper = &exportCSVHelper{
				spaceID:              spaceID,
				exptID:               exptID,
				withLogID:            withLogID,
				colSelection:         sel,
				reportEvaluatorCount: len(result.ColumnEvaluators),
				colEvaluators:        filterColumnEvaluatorsForExport(result.ColumnEvaluators, sel),
				colEvalSetFields:     filterColumnEvalSetFieldsForExport(result.ColumnEvalSetFields, sel),
				colAnnotations:       filterColumnAnnotationsForExport(colAnnotation, sel),
				columnsEvalTarget:    columnsEvalTarget,
				exptRepo:             e.exptRepo,
				exptTurnResultRepo:   e.exptTurnResultRepo,
				exptPublisher:        e.exptPublisher,
				exptResultService:    e.exptResultService,
				fileClient:           e.fileClient,
				evalSetItemSvc:       e.evalSetItemSvc,
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

		if result.NextTurnListCursor == nil {
			break
		}
		turnCursor = result.NextTurnListCursor
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

	colSelection         *exportColumnSelection
	reportEvaluatorCount int

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
		name, ver := ptr.From(colEvaluator.Name), ptr.From(colEvaluator.Version)
		if e.colSelection == nil || e.colSelection.exportAll {
			columns = append(columns, getColumnNameEvaluator(name, ver), getColumnNameEvaluatorReason(name, ver))
			continue
		}
		if e.colSelection.includeEvaluatorScore(colEvaluator.EvaluatorVersionID) {
			columns = append(columns, getColumnNameEvaluator(name, ver))
		}
		if e.colSelection.includeEvaluatorReason(colEvaluator.EvaluatorVersionID) {
			columns = append(columns, getColumnNameEvaluatorReason(name, ver))
		}
	}

	if e.wantWeightedScoreColumn() {
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

func (e *exportCSVHelper) wantWeightedScoreColumn() bool {
	if e.reportEvaluatorCount == 0 {
		return false
	}
	if e.colSelection == nil || e.colSelection.exportAll {
		return true
	}
	return e.colSelection.includeWeightedScore()
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
				if e.colSelection == nil || e.colSelection.exportAll {
					rowData = append(rowData, getEvaluatorScore(evaluatorRecord), getEvaluatorReason(evaluatorRecord))
					continue
				}
				if e.colSelection.includeEvaluatorScore(colEvaluator.EvaluatorVersionID) {
					rowData = append(rowData, getEvaluatorScore(evaluatorRecord))
				}
				if e.colSelection.includeEvaluatorReason(colEvaluator.EvaluatorVersionID) {
					rowData = append(rowData, getEvaluatorReason(evaluatorRecord))
				}
			}

			if e.wantWeightedScoreColumn() {
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
			// 必须与表头「每列一条」对齐；不能 continue 少一格，否则后续 Target 等列整体错位，表现为空数据且表头与列对不上。
			fields = append(fields, "")
			continue
		}

		if fieldData.Content.IsContentOmitted() {
			logs.CtxInfo(ctx, "ContentOmitted fieldData: %v", json.Jsonify(fieldData))
			if fieldData, err = e.evalSetItemSvc.GetEvaluationSetItemField(ctx, &entity.GetEvaluationSetItemFieldParam{
				SpaceID:         e.spaceID,
				EvaluationSetID: tes.EvalSetID,
				ItemPK:          tes.ItemID,
				FieldName:       gptr.Indirect(colEvalSetField.Name),
				FieldKey:        colEvalSetField.Key,
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

func cloneExptExportColumnSpec(src *entity.ExptResultExportColumnSpec) *entity.ExptResultExportColumnSpec {
	if src == nil {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return src
	}
	var dst entity.ExptResultExportColumnSpec
	if err = json.Unmarshal(raw, &dst); err != nil {
		return src
	}
	return &dst
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
