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
}

func NewInsightAnalysisService(repo repo.IExptInsightAnalysisRecordRepo,
	exptPublisher events.ExptEventPublisher,
	fileClient fileserver.ObjectStorage,
	agentAdapter rpc.IAgentAdapter,
	exptResultExportService IExptResultExportService,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	exptRepo repo.IExperimentRepo,
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
		CreateAt:     time.Now().Unix(),
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
			AnalysisReportID:   ptr.Of(analysisReportID),
			Status:             entity.InsightAnalysisStatus_Running,
		}
		if err != nil {
			record.Status = entity.InsightAnalysisStatus_Failed
			// 不重试
			err = nil
		}
		err1 := e.repo.UpdateAnalysisRecord(ctx, record)
		if err1 != nil {
			logs.CtxError(ctx, "UpdateAnalysisRecord failed: %v", err1)
			return
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
	logs.CtxInfo(ctx, "GenAnalysisReport get csv url=%v", url)

	reportID, err := e.agentAdapter.CallTraceAgent(ctx, spaceID, url)
	if err != nil {
		return err
	}
	logs.CtxInfo(ctx, "[GenAnalysisReport] CallTraceAgent success, expt_id=%v, record_id=%v, report_id=%v, csv url=%v", exptID, recordID, reportID, url)

	analysisReportID = reportID

	// 发送时间检查分析报告生成状态
	exportEvent := &entity.ExportCSVEvent{
		ExportID:     recordID,
		ExperimentID: exptID,
		SpaceID:      spaceID,
		ExportScene:  entity.ExportSceneInsightAnalysis,
		CreateAt:     CreateAt,
	}
	err = e.exptPublisher.PublishExptExportCSVEvent(ctx, exportEvent, gptr.Of(time.Minute*3))
	if err != nil {
		return err
	}

	return nil
}

func (e ExptInsightAnalysisServiceImpl) checkAnalysisReportGenStatus(ctx context.Context, record *entity.ExptInsightAnalysisRecord, CreateAt int64) (err error) {
	_, status, err := e.agentAdapter.GetReport(ctx, record.SpaceID, ptr.From(record.AnalysisReportID))
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

	defaultIntervalSecond := 60 * 60 * 1
	if time.Now().Unix()-CreateAt >= int64(defaultIntervalSecond) {
		logs.CtxWarn(ctx, "checkAnalysisReportGenStatus found timeout event, expt_id: %v, record_id: %v", record.ExptID, record.ID)
		record.Status = entity.InsightAnalysisStatus_Failed
		return e.repo.UpdateAnalysisRecord(ctx, record)
	}

	exportEvent := &entity.ExportCSVEvent{
		ExportID:     record.ID,
		ExperimentID: record.ExptID,
		SpaceID:      record.SpaceID,
		ExportScene:  entity.ExportSceneInsightAnalysis,
		CreateAt:     CreateAt,
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

	if analysisRecord.Status == entity.InsightAnalysisStatus_Running ||
		analysisRecord.Status == entity.InsightAnalysisStatus_Failed {
		return analysisRecord, nil
	}

	report, _, err := e.agentAdapter.GetReport(ctx, spaceID, ptr.From(analysisRecord.AnalysisReportID))
	if err != nil {
		return nil, err
	}

	analysisRecord.AnalysisReportContent = report

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

func (e ExptInsightAnalysisServiceImpl) notifyAnalysisComplete(ctx context.Context, userID string, spaceID, exptID int64) error {
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	userInfos, err := e.userProvider.MGetUserInfo(ctx, []string{userID})
	if err != nil {
		return err
	}
	logs.CtxInfo(ctx, "notifyAnalysisComplete userInfos: %v", userInfos)

	if len(userInfos) != 1 || userInfos[0] == nil {
		return nil
	}

	userInfo := userInfos[0]
	logs.CtxInfo(ctx, "notifyAnalysisComplete userInfo: %v", userInfo)
	const cardID = "AAq9DvIYd2qHu"
	err = e.notifyRPCAdapter.SendMessageCard(ctx, ptr.From(userInfo.Email), cardID, map[string]string{
		"expt_name": expt.Name,
		"space_id":  strconv.FormatInt(spaceID, 10),
		"expt_id":   strconv.FormatInt(exptID, 10),
	})

	return err
}

func (e ExptInsightAnalysisServiceImpl) ListAnalysisRecord(ctx context.Context, spaceID, exptID int64, page entity.Page, session *entity.Session) ([]*entity.ExptInsightAnalysisRecord, int64, error) {
	return e.repo.ListAnalysisRecord(ctx, spaceID, exptID, page)
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
		return e.repo.DeleteFeedbackComment(ctx, param.SpaceID, param.ExptID, ptr.From(param.CommentID))
	default:
		return nil
	}
}

func (e ExptInsightAnalysisServiceImpl) ListExptInsightAnalysisFeedbackComment(ctx context.Context, spaceID, exptID, recordID int64, page entity.Page) ([]*entity.ExptInsightAnalysisFeedbackComment, int64, error) {
	return e.repo.List(ctx, spaceID, exptID, recordID, page)
}
