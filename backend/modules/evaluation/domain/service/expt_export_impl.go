// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

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

		url, _, err := e.fileClient.SignDownloadReq(ctx, exportRecord.FilePath, signOpt)
		if err != nil {
			return nil, err
		}

		exportRecord.URL = ptr.Of(url)
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

func (e ExptResultExportService) DoExportCSV(ctx context.Context, spaceID, exptID, exportID int64) (err error) {
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
			return
		}
	}()

	var (
		pageNum  = 1
		pageSize = 100
		// total    int64
		maxPage = 500

		colEvaluators    []*entity.ColumnEvaluator
		colEvalSetFields []*entity.ColumnEvalSetField
		colAnnotation    []*entity.ColumnAnnotation
		allItemResults   []*entity.ItemResult
	)

	for {
		page := entity.NewPage(pageNum, pageSize)
		param := &entity.MGetExperimentResultParam{
			SpaceID:    spaceID,
			ExptIDs:    []int64{exptID},
			BaseExptID: ptr.Of(exptID),
			Page:       page,
		}
		columnEvaluators, _, columnEvalSetFields, exptColumnAnnotation, itemResults, total, err := e.exptResultService.MGetExperimentResult(ctx, param)
		if err != nil {
			return err
		}

		colEvaluators = columnEvaluators
		colEvalSetFields = columnEvalSetFields
		for _, columnAnnotation := range exptColumnAnnotation {
			if columnAnnotation.ExptID == exptID {
				colAnnotation = columnAnnotation.ColumnAnnotations
			}
		}
		allItemResults = append(allItemResults, itemResults...)

		if pageNum*pageSize >= int(total) {
			break
		}

		if pageNum > maxPage {
			break
		}

		pageNum++
	}

	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	fileName, err = e.getFileName(ctx, expt.Name, exportID)
	if err != nil {
		return err
	}

	exportHelper := &exportCSVHelper{
		exportID:           exportID,
		exptID:             exptID,
		spaceID:            spaceID,
		exptRepo:           e.exptRepo,
		exptTurnResultRepo: e.exptTurnResultRepo,
		exptPublisher:      e.exptPublisher,
		exptResultService:  e.exptResultService,
		fileClient:         e.fileClient,
		fileName:           fileName,

		colEvaluators:    colEvaluators,
		colAnnotations:   colAnnotation,
		colEvalSetFields: colEvalSetFields,
		allItemResults:   allItemResults,
	}

	err = exportHelper.exportCSV(ctx)
	if err != nil {
		return err
	}

	return nil
}

type exportCSVHelper struct {
	exportID int64
	spaceID  int64
	exptID   int64
	fileName string

	colEvaluators    []*entity.ColumnEvaluator
	colEvalSetFields []*entity.ColumnEvalSetField
	colAnnotations   []*entity.ColumnAnnotation
	allItemResults   []*entity.ItemResult

	exptRepo           repo.IExperimentRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptPublisher      events.ExptEventPublisher
	exptResultService  ExptResultService
	fileClient         fileserver.ObjectStorage
}

func (e *exportCSVHelper) exportCSV(ctx context.Context) error {
	// 表头信息
	columns, err := e.buildColumns(ctx)
	if err != nil {
		return err
	}

	// 数据信息
	fileData := make([][]string, 0)

	rows, err := e.buildRows(ctx)
	if err != nil {
		return err
	}

	// 合并表头和数据
	fileData = append(fileData, columns)
	fileData = append(fileData, rows...)

	err = e.createAndUploadCSV(ctx, e.fileName, fileData)
	if err != nil {
		return err
	}

	return nil
}

const (
	columnNameID     = "ID"
	columnNameStatus = "status"
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

	// 实际输出
	columns = append(columns, consts.OutputSchemaKey)

	// colEvaluators
	for _, colEvaluator := range e.colEvaluators {
		if colEvaluator == nil {
			continue
		}

		columns = append(columns, getColumnNameEvaluator(ptr.From(colEvaluator.Name), ptr.From(colEvaluator.Version)))
		columns = append(columns, getColumnNameEvaluatorReason(ptr.From(colEvaluator.Name), ptr.From(colEvaluator.Version)))
	}

	// colAnnotations
	for _, colAnnotation := range e.colAnnotations {
		if colAnnotation == nil {
			continue
		}

		columns = append(columns, colAnnotation.TagName)

	}

	return columns, nil
}

func getColumnNameEvaluator(evaluatorName, version string) string {
	return fmt.Sprintf("%s<%s>", evaluatorName, version)
}

func getColumnNameEvaluatorReason(evaluatorName, version string) string {
	return fmt.Sprintf("%s<%s>_reason", evaluatorName, version)
}

func (e *exportCSVHelper) buildRows(ctx context.Context) ([][]string, error) {
	rows := make([][]string, 0)
	for _, itemResult := range e.allItemResults {
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
			datasetFields := getDatasetFields(e.colEvalSetFields, payload.EvalSet.Turn.FieldDataList)
			rowData = append(rowData, datasetFields...)

			// 实际输出
			var actualOutput string
			if payload.TargetOutput == nil ||
				payload.TargetOutput.EvalTargetRecord == nil ||
				payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil ||
				payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields == nil {
				actualOutput = ""
			} else {
				actualOutput = geDatasetCellOrActualOutputData(payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields[consts.OutputSchemaKey])
			}
			rowData = append(rowData, actualOutput)

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
func getDatasetFields(colEvalSetFields []*entity.ColumnEvalSetField, fieldDataList []*entity.FieldData) []string {
	fieldDataMap := slices.ToMap(fieldDataList, func(t *entity.FieldData) (string, *entity.FieldData) {
		return t.Key, t
	})
	fields := make([]string, 0, len(colEvalSetFields))
	for _, colEvalSetField := range colEvalSetFields {
		if colEvalSetField == nil {
			continue
		}

		fieldData, ok := fieldDataMap[ptr.From(colEvalSetField.Key)]
		if !ok {
			fields = append(fields, "")
			continue
		}

		fields = append(fields, geDatasetCellOrActualOutputData(fieldData.Content))
	}

	return fields
}

func geDatasetCellOrActualOutputData(data *entity.Content) string {
	if data == nil {
		return ""
	}

	switch data.GetContentType() {
	case entity.ContentTypeText:
		return data.GetText()
	case entity.ContentTypeImage, entity.ContentTypeAudio:
		return ""
	case entity.ContentTypeMultipart:
		return formatMultiPartData(data)
	default:
		return ""
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
		case entity.ContentTypeAudio, entity.ContentTypeMultipart:
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

func (e *exportCSVHelper) createAndUploadCSV(ctx context.Context, fileName string, fileData [][]string) error {
	err := e.createCSV(ctx, fileName, fileData)
	if err != nil {
		return err
	}
	// 上传文件

	csvFile, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer func() {
		_ = csvFile.Close()
	}()

	fileReader := bufio.NewReader(csvFile)

	err = e.uploadCSVFile(ctx, fileName, fileReader)
	if err != nil {
		return fmt.Errorf("uploadFile error: %v", err)
	}

	// 删除CSV文件
	err = os.Remove(fileName)
	if err != nil {
		return err
	}

	return nil
}

func (e *exportCSVHelper) createCSV(ctx context.Context, fileName string, fileData [][]string) error {
	// 创建CSV文件
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = file.WriteString("\xEF\xBB\xBF") // 写入UTF-8 BOM，避免使用Excel打开乱码
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 将数据写入CSV文件
	for _, rowData := range fileData {
		err := writer.Write(rowData)
		if err != nil {
			return err
		}
	}

	logs.CtxInfo(ctx, "CSV file successfully created, file = %v", file.Name())
	return nil
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
