// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptInsightAnalysisServiceImpl struct {
	repo                    repo.IExptInsightAnalysisRecordRepo
	exptPublisher           events.ExptEventPublisher
	fileClient              fileserver.ObjectStorage
	agentAdapter            rpc.IAgentAdapter
	exptResultExportService IExptResultExportService
	notifyRPCAdapter        rpc.INotifyRPCAdapter
	userProvider            rpc.IUserProvider
	exptRepo                repo.IExperimentRepo
	targetRepo              repo.IEvalTargetRepo
}

func NewInsightAnalysisService(repo repo.IExptInsightAnalysisRecordRepo,
	exptPublisher events.ExptEventPublisher,
	fileClient fileserver.ObjectStorage,
	agentAdapter rpc.IAgentAdapter,
	exptResultExportService IExptResultExportService,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	exptRepo repo.IExperimentRepo,
	targetRepo repo.IEvalTargetRepo,
) IExptInsightAnalysisService {
	return &ExptInsightAnalysisServiceImpl{
		repo:                    repo,
		exptPublisher:           exptPublisher,
		fileClient:              fileClient,
		agentAdapter:            agentAdapter,
		exptResultExportService: exptResultExportService,
		notifyRPCAdapter:        notifyRPCAdapter,
		userProvider:            userProvider,
		exptRepo:                exptRepo,
		targetRepo:              targetRepo,
	}
}

func (e ExptInsightAnalysisServiceImpl) CreateAnalysisRecord(ctx context.Context, record *entity.ExptInsightAnalysisRecord, session *entity.Session) (int64, error) {
	recordID, err := e.repo.CreateAnalysisRecord(ctx, record)
	if err != nil {
		return 0, err
	}

	exportEvent := &entity.ExportCSVEvent{
		ExportID:     recordID,
		ExperimentID: record.ExptID,
		SpaceID:      record.SpaceID,
		ExportScene:  entity.ExportSceneInsightAnalysis,
		CreatedAt:    time.Now().Unix(),
	}
	err = e.exptPublisher.PublishExptExportCSVEvent(ctx, exportEvent, gptr.Of(time.Second*3))
	if err != nil {
		return 0, err
	}

	return recordID, nil
}

func (e ExptInsightAnalysisServiceImpl) GenAnalysisReport(ctx context.Context, spaceID, exptID, recordID, CreateAt int64) (err error) {
	analysisRecord, err := e.repo.GetAnalysisRecordByID(ctx, spaceID, exptID, recordID)
	if err != nil {
		return err
	}
	if analysisRecord.AnalysisReportID != nil {
		return e.checkAnalysisReportGenStatus(ctx, analysisRecord, CreateAt)
	}

	var (
		exptResultFilePath string
		analysisReportID   int64
	)
	defer func() {
		record := &entity.ExptInsightAnalysisRecord{
			ID:                 recordID,
			SpaceID:            spaceID,
			ExptID:             exptID,
			ExptResultFilePath: ptr.Of(exptResultFilePath),
			Status:             entity.InsightAnalysisStatus_Running,
		}
		if analysisReportID > 0 {
			record.AnalysisReportID = ptr.Of(analysisReportID)
		}
		if err != nil {
			record.Status = entity.InsightAnalysisStatus_Failed
		}
		err1 := e.repo.UpdateAnalysisRecord(ctx, record)
		if err1 != nil {
			logs.CtxError(ctx, "UpdateAnalysisRecord failed: %v", err1)
			err = err1
		}
	}()

	fileName := fmt.Sprintf("insight_analysis_%d_%d.csv", spaceID, recordID)
	exptResultFilePath = fileName
	err = e.exptResultExportService.DoExportCSV(ctx, spaceID, exptID, fileName, true)
	if err != nil {
		return err
	}

	var ttl int64 = 24 * 60 * 60
	signOpt := fileserver.SignWithTTL(time.Duration(ttl) * time.Second)

	url, _, err := e.fileClient.SignDownloadReq(ctx, fileName, signOpt)
	if err != nil {
		return err
	}

	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}

	param := &rpc.CallTraceAgentParam{
		SpaceID:        spaceID,
		ExptID:         exptID,
		Url:            url,
		EvalTargetType: expt.TargetType,
	}

	if expt.StartAt == nil || expt.EndAt == nil {
		logs.CtxWarn(ctx, "Experiment %d has no start or end time", exptID)
	} else {
		param.StartTime = expt.StartAt.UnixMilli()
		param.EndTime = expt.EndAt.UnixMilli()
	}

	target, err := e.targetRepo.GetEvalTargetVersion(ctx, spaceID, expt.TargetVersionID)
	if err != nil {
		return err
	}
	if target == nil || target.SourceTargetID == "" {
		logs.CtxWarn(ctx, "Experiment %d has no source target %d", exptID, expt.TargetID)
		return errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg(fmt.Sprintf("Experiment %d has no source target %d", exptID, expt.TargetID)))
	}
	param.EvalTargetID, err = strconv.ParseInt(target.SourceTargetID, 10, 64)
	if err != nil {
		return err
	}
	if target.EvalTargetVersion == nil || target.EvalTargetVersion.SourceTargetVersion == "" {
		logs.CtxWarn(ctx, "Experiment %d has no source target version %s", exptID, expt.TargetVersionID)
		return errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg(fmt.Sprintf("Experiment %d has no source target version %d", exptID, expt.TargetVersionID)))
	}
	param.EvalTargetVersion = target.EvalTargetVersion.SourceTargetVersion

	evaluators, err := e.exptRepo.GetEvaluatorRefByExptIDs(ctx, []int64{exptID}, spaceID)
	if err != nil {
		return err
	}
	param.Evaluators = evaluators

	// only allow prompt eval target, but not return error here. The task will fail in the CallTraceAgent.
	if param.EvalTargetType != entity.EvalTargetTypeLoopPrompt {
		logs.CtxWarn(ctx, "Illegal evaltarget type %d for expt %d", param.EvalTargetType, exptID)
	}

	reportID, err := e.agentAdapter.CallTraceAgent(ctx, param)
	if err != nil {
		return err
	}
	logs.CtxInfo(ctx, "[GenAnalysisReport] CallTraceAgent success, expt_id=%v, record_id=%v, report_id=%v", exptID, recordID, reportID)

	analysisReportID = reportID

	// 发送时间检查分析报告生成状态
	exportEvent := &entity.ExportCSVEvent{
		ExportID:     recordID,
		ExperimentID: exptID,
		SpaceID:      spaceID,
		ExportScene:  entity.ExportSceneInsightAnalysis,
		CreatedAt:    CreateAt,
	}
	err = e.exptPublisher.PublishExptExportCSVEvent(ctx, exportEvent, gptr.Of(time.Minute*3))
	if err != nil {
		return err
	}

	return nil
}

func (e ExptInsightAnalysisServiceImpl) checkAnalysisReportGenStatus(ctx context.Context, record *entity.ExptInsightAnalysisRecord, CreateAt int64) (err error) {
	_, _, status, err := e.agentAdapter.GetReport(ctx, record.SpaceID, ptr.From(record.AnalysisReportID))
	if err != nil {
		return err
	}

	if status == entity.ReportStatus_Failed {
		record.Status = entity.InsightAnalysisStatus_Failed
		return e.repo.UpdateAnalysisRecord(ctx, record)
	}
	if status == entity.ReportStatus_Success {
		err = e.notifyAnalysisComplete(ctx, record.CreatedBy, record.SpaceID, record.ExptID)
		if err != nil {
			logs.CtxWarn(ctx, "notifyAnalysisComplete failed, err=%v", err)
		}
		record.Status = entity.InsightAnalysisStatus_Success
		return e.repo.UpdateAnalysisRecord(ctx, record)
	}

	// 超过2小时，未生成分析报告，认为是失败
	if status == entity.ReportStatus_Running && record.CreatedAt.Add(entity.InsightAnalysisRunningTimeout).Unix() <= time.Now().Unix() {
		record.Status = entity.InsightAnalysisStatus_Failed
		logs.CtxWarn(ctx, "checkAnalysisReportGenStatus found timeout event, space_id: %v, expt_id: %v, record_id: %v, report_id: %v", record.SpaceID, record.ExptID, record.ID, gptr.Indirect(record.AnalysisReportID))
		return e.repo.UpdateAnalysisRecord(ctx, record)
	}

	exportEvent := &entity.ExportCSVEvent{
		ExportID:     record.ID,
		ExperimentID: record.ExptID,
		SpaceID:      record.SpaceID,
		ExportScene:  entity.ExportSceneInsightAnalysis,
		CreatedAt:    CreateAt,
	}
	err = e.exptPublisher.PublishExptExportCSVEvent(ctx, exportEvent, gptr.Of(time.Minute*1))
	if err != nil {
		return err
	}

	return nil
}

func (e ExptInsightAnalysisServiceImpl) GetAnalysisRecordByID(ctx context.Context, spaceID, exptID, recordID int64, session *entity.Session) (*entity.ExptInsightAnalysisRecord, error) {
	analysisRecord, err := e.repo.GetAnalysisRecordByID(ctx, spaceID, exptID, recordID)
	if err != nil {
		return nil, err
	}

	if analysisRecord.Status == entity.InsightAnalysisStatus_Running && analysisRecord.CreatedAt.Add(entity.InsightAnalysisRunningTimeout).Unix() < time.Now().Unix() {
		analysisRecord.Status = entity.InsightAnalysisStatus_Failed
		err = e.repo.UpdateAnalysisRecord(ctx, analysisRecord)
		if err != nil {
			logs.CtxError(ctx, "GetAnalysisRecordByID: UpdateAnalysisRecord failed: %v", err)
		}
		return analysisRecord, err
	}

	if analysisRecord.Status == entity.InsightAnalysisStatus_Running ||
		analysisRecord.Status == entity.InsightAnalysisStatus_Failed {
		return analysisRecord, nil
	}

	report, reportIdx, _, err := e.agentAdapter.GetReport(ctx, spaceID, ptr.From(analysisRecord.AnalysisReportID))
	if err != nil {
		return nil, err
	}

	analysisRecord.AnalysisReportContent = report
	analysisRecord.AnalysisReportIndex = reportIdx

	upvoteCount, downvoteCount, err := e.repo.CountFeedbackVote(ctx, spaceID, exptID, recordID)
	if err != nil {
		return nil, err
	}

	curUserFeedbackVote, err := e.repo.GetFeedbackVoteByUser(ctx, spaceID, exptID, recordID, session.UserID)
	if err != nil {
		return nil, err
	}
	analysisRecord.ExptInsightAnalysisFeedback = entity.ExptInsightAnalysisFeedback{
		UpvoteCount:         upvoteCount,
		DownvoteCount:       downvoteCount,
		CurrentUserVoteType: entity.None,
	}

	if curUserFeedbackVote != nil {
		analysisRecord.ExptInsightAnalysisFeedback.CurrentUserVoteType = curUserFeedbackVote.VoteType
	}

	return analysisRecord, nil
}

func (e ExptInsightAnalysisServiceImpl) GetAnalysisRecordFeedbackVoteByUser(ctx context.Context, spaceID, exptID, recordID int64, session *entity.Session) (*entity.ExptInsightAnalysisFeedbackVote, error) {
	if session == nil || session.UserID == "" {
		return nil, nil
	}

	vote, err := e.repo.GetFeedbackVoteByUser(ctx, spaceID, exptID, recordID, session.UserID)
	if err != nil {
		return nil, err
	}

	return vote, nil
}

func (e ExptInsightAnalysisServiceImpl) notifyAnalysisComplete(ctx context.Context, userID string, spaceID, exptID int64) error {
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	userInfos, err := e.userProvider.MGetUserInfo(ctx, []string{userID})
	if err != nil {
		return err
	}

	if len(userInfos) != 1 || userInfos[0] == nil {
		return nil
	}

	userInfo := userInfos[0]
	err = e.notifyRPCAdapter.SendMessageCard(ctx, ptr.From(userInfo.Email), consts.InsightAnalysisNotifyCardID, map[string]string{
		"expt_name": expt.Name,
		"space_id":  strconv.FormatInt(spaceID, 10),
		"expt_id":   strconv.FormatInt(exptID, 10),
	})

	return err
}

func (e ExptInsightAnalysisServiceImpl) ListAnalysisRecord(ctx context.Context, spaceID, exptID int64, page entity.Page, session *entity.Session) ([]*entity.ExptInsightAnalysisRecord, int64, error) {
	analysisRecords, total, err := e.repo.ListAnalysisRecord(ctx, spaceID, exptID, page)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return analysisRecords, total, nil
	}

	firstAnalysisRecord := analysisRecords[0]

	upvoteCount, downvoteCount, err := e.repo.CountFeedbackVote(ctx, spaceID, exptID, firstAnalysisRecord.ID)
	if err != nil {
		// side path, don't block the main flow
		logs.CtxWarn(ctx, "CountFeedbackVote failed for space_id: %v, expt_id: %v, record_id: %v, err=%v", spaceID, exptID, firstAnalysisRecord.ID, err)
		return analysisRecords, total, nil
	}

	curUserFeedbackVote, err := e.repo.GetFeedbackVoteByUser(ctx, spaceID, exptID, firstAnalysisRecord.ID, session.UserID)
	if err != nil {
		// side path, don't block the main flow
		logs.CtxWarn(ctx, "GetFeedbackVoteByUser failed for space_id: %v, expt_id: %v, record_id: %v, err=%v", spaceID, exptID, firstAnalysisRecord.ID, err)
		return analysisRecords, total, nil
	}
	firstAnalysisRecord.ExptInsightAnalysisFeedback = entity.ExptInsightAnalysisFeedback{
		UpvoteCount:         upvoteCount,
		DownvoteCount:       downvoteCount,
		CurrentUserVoteType: entity.None,
	}
	firstAnalysisRecord.ExptInsightAnalysisFeedback.CurrentUserVoteType = entity.None
	if curUserFeedbackVote != nil {
		firstAnalysisRecord.ExptInsightAnalysisFeedback.CurrentUserVoteType = curUserFeedbackVote.VoteType
	}

	return analysisRecords, total, nil
}

func (e ExptInsightAnalysisServiceImpl) DeleteAnalysisRecord(ctx context.Context, spaceID, exptID, recordID int64) error {
	return e.repo.DeleteAnalysisRecord(ctx, spaceID, exptID, recordID)
}

func (e ExptInsightAnalysisServiceImpl) FeedbackExptInsightAnalysis(ctx context.Context, param *entity.ExptInsightAnalysisFeedbackParam) error {
	if param.Session == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("empty session"))
	}
	switch param.FeedbackActionType {
	case entity.FeedbackActionType_Upvote:
		feedbackVote := &entity.ExptInsightAnalysisFeedbackVote{
			SpaceID:          param.SpaceID,
			ExptID:           param.ExptID,
			AnalysisRecordID: param.AnalysisRecordID,
			CreatedBy:        param.Session.UserID,
			VoteType:         entity.Upvote,
		}
		return e.repo.CreateFeedbackVote(ctx, feedbackVote)
	case entity.FeedbackActionType_CancelUpvote, entity.FeedbackActionType_CancelDownvote:
		feedbackVote := &entity.ExptInsightAnalysisFeedbackVote{
			SpaceID:          param.SpaceID,
			ExptID:           param.ExptID,
			AnalysisRecordID: param.AnalysisRecordID,
			CreatedBy:        param.Session.UserID,
			VoteType:         entity.None,
		}
		return e.repo.UpdateFeedbackVote(ctx, feedbackVote)
	case entity.FeedbackActionType_Downvote:
		feedbackVote := &entity.ExptInsightAnalysisFeedbackVote{
			SpaceID:          param.SpaceID,
			ExptID:           param.ExptID,
			AnalysisRecordID: param.AnalysisRecordID,
			CreatedBy:        param.Session.UserID,
			VoteType:         entity.Downvote,
		}
		return e.repo.CreateFeedbackVote(ctx, feedbackVote)
	case entity.FeedbackActionType_CreateComment:
		feedbackComment := &entity.ExptInsightAnalysisFeedbackComment{
			SpaceID:          param.SpaceID,
			ExptID:           param.ExptID,
			AnalysisRecordID: param.AnalysisRecordID,
			CreatedBy:        param.Session.UserID,
			Comment:          ptr.From(param.Comment),
		}
		return e.repo.CreateFeedbackComment(ctx, feedbackComment)
	case entity.FeedbackActionType_Update_Comment:
		if param.CommentID == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("empty comment_id"))
		}
		feedbackComment := &entity.ExptInsightAnalysisFeedbackComment{
			ID:               ptr.From(param.CommentID),
			SpaceID:          param.SpaceID,
			ExptID:           param.ExptID,
			AnalysisRecordID: param.AnalysisRecordID,
			CreatedBy:        param.Session.UserID,
			Comment:          ptr.From(param.Comment),
		}
		return e.repo.UpdateFeedbackComment(ctx, feedbackComment)
	case entity.FeedbackActionType_Delete_Comment:
		if param.CommentID == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("empty comment_id"))
		}
		return e.repo.DeleteFeedbackComment(ctx, param.SpaceID, param.ExptID, ptr.From(param.CommentID))
	default:
		return nil
	}
}

func (e ExptInsightAnalysisServiceImpl) ListExptInsightAnalysisFeedbackComment(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*entity.ExptInsightAnalysisFeedbackComment, int64, error) {
	return e.repo.List(ctx, spaceID, exptID, recordID, page)
}
