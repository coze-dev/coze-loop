// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
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
}

func NewInsightAnalysisService(repo repo.IExptInsightAnalysisRecordRepo,
	exptPublisher events.ExptEventPublisher,
	fileClient fileserver.ObjectStorage,
	agentAdapter rpc.IAgentAdapter,
	exptResultExportService IExptResultExportService,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider) IExptInsightAnalysisService {
	return &ExptInsightAnalysisServiceImpl{
		repo:                    repo,
		exptPublisher:           exptPublisher,
		fileClient:              fileClient,
		agentAdapter:            agentAdapter,
		exptResultExportService: exptResultExportService,
		notifyRPCAdapter:        notifyRPCAdapter,
		userProvider:            userProvider,
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
	}
	err = e.exptPublisher.PublishExptExportCSVEvent(ctx, exportEvent, gptr.Of(time.Second*3))
	if err != nil {
		return 0, err
	}

	//time.Sleep(time.Second)
	//err = e.GenAnalysisReport(ctx, record.SpaceID, record.ExptID, recordID)
	//if err != nil {
	//	logs.CtxError(ctx, "GenAnalysisReport err: %v", err)
	//}

	return recordID, nil
}

func (e ExptInsightAnalysisServiceImpl) GenAnalysisReport(ctx context.Context, spaceID, exptID, recordID int64) (err error) {
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
			Status:             entity.InsightAnalysisStatus_Success,
		}
		if err != nil {
			record.Status = entity.InsightAnalysisStatus_Failed
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

	analysisReportID = reportID

	return nil
}

func (e ExptInsightAnalysisServiceImpl) GetAnalysisRecordByID(ctx context.Context, spaceID, exptID, recordID int64, session *entity.Session) (*entity.ExptInsightAnalysisRecord, error) {
	err := e.notifyAnalysisComplete(ctx, session.UserID)
	if err != nil {
		logs.CtxWarn(ctx, "notifyAnalysisComplete failed, err=%v", err)
	}

	analysisRecord, err := e.repo.GetAnalysisRecordByID(ctx, spaceID, exptID, recordID)
	if err != nil {
		return nil, err
	}

	if analysisRecord.Status != entity.InsightAnalysisStatus_Success {
		return analysisRecord, nil
	}

	report, status, err := e.agentAdapter.GetReport(ctx, spaceID, ptr.From(analysisRecord.AnalysisReportID))
	if err != nil {
		return nil, err
	}
	// 聚合报告生成状态
	if status == entity.ReportStatus_Failed {
		analysisRecord.Status = entity.InsightAnalysisStatus_Failed
		return analysisRecord, nil
	}
	if status == entity.ReportStatus_Running {
		analysisRecord.Status = entity.InsightAnalysisStatus_Running
		return analysisRecord, nil
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

	//err = e.notifyAnalysisComplete(ctx, session.UserID)
	//if err != nil {
	//	logs.CtxWarn(ctx, "notifyAnalysisComplete failed, err=%v", err)
	//}

	return analysisRecord, nil
}

func (e ExptInsightAnalysisServiceImpl) notifyAnalysisComplete(ctx context.Context, userID string) error {
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

	err = e.notifyRPCAdapter.SendLarkMessageCard(ctx, ptr.From(userInfo.), "AAq9DvIYd2qHu", map[string]string{
		"expt_name": "实验名称",
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
