// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type WebhookDeliveryConsumer struct {
	webhookSender componentwebhook.IWebhookSender
	deliveryRepo  repo.IWebhookDeliveryRepo
	publisher     events.WebhookDeliveryEventPublisher
	configer      component.IWebhookConfiger
	exptRepo      repo.IExperimentRepo
	resultSvc     service.ExptResultService
	aggrResultSvc service.ExptAggrResultService
}

func NewWebhookDeliveryConsumer(
	webhookSender componentwebhook.IWebhookSender,
	deliveryRepo repo.IWebhookDeliveryRepo,
	publisher events.WebhookDeliveryEventPublisher,
	configer component.IWebhookConfiger,
	exptRepo repo.IExperimentRepo,
	resultSvc service.ExptResultService,
	aggrResultSvc service.ExptAggrResultService,
) mq.IConsumerHandler {
	return &WebhookDeliveryConsumer{
		webhookSender: webhookSender,
		deliveryRepo:  deliveryRepo,
		publisher:     publisher,
		configer:      configer,
		exptRepo:      exptRepo,
		resultSvc:     resultSvc,
		aggrResultSvc: aggrResultSvc,
	}
}

func (c *WebhookDeliveryConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) error {
	event := &entity.WebhookDeliveryMessage{}
	if err := sonic.Unmarshal(ext.Body, event); err != nil {
		logs.CtxError(ctx, "[Webhook] delivery message unmarshal fail, msg_id: %v, raw: %v, err: %v", ext.MsgID, string(ext.Body), err)
		return nil
	}
	logs.CtxInfo(ctx, "[Webhook] consume delivery message, msg_id: %v, delivery_id: %v, attempt: %v", ext.MsgID, event.DeliveryID, event.Attempt)

	webhookConf := c.configer.GetWebhookConf(ctx)
	if !webhookConf.IsEnabled(event.SpaceID) {
		logs.CtxInfo(ctx, "[Webhook] disabled, skip delivery, delivery_id: %v, space_id: %v", event.DeliveryID, event.SpaceID)
		return nil
	}
	secret := webhookConf.GetSigningSecret(event.SpaceID)
	if secret == "" {
		logs.CtxError(ctx, "[Webhook] delivery secret is empty, delivery_id: %v, space_id: %v", event.DeliveryID, event.SpaceID)
		return c.markFailed(ctx, event, nil, "webhook signing secret is empty")
	}
	retryConf := c.configer.GetWebhookRetryConf(ctx)
	if event.CreatedAt > 0 && retryConf.MessageTTL > 0 && time.Since(time.Unix(event.CreatedAt, 0)) > retryConf.MessageTTL {
		return c.markFailed(ctx, event, nil, "webhook delivery message expired")
	}

	expt, err := c.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "[Webhook] get expt fail, delivery_id: %v, expt_id: %v, err: %v", event.DeliveryID, event.ExptID, err)
		return c.markFailed(ctx, event, nil, fmt.Sprintf("get experiment failed: %v", err))
	}
	c.enrichWebhookExperiment(ctx, event.SpaceID, expt)
	payload := buildWebhookPayload(event, expt, webhookConf)
	result := c.webhookSender.Send(ctx, event.WebhookURL, payload, secret)
	if result != nil && result.Success {
		return c.markSuccess(ctx, event, result.StatusCode)
	}
	if event.Attempt < retryConf.MaxRetries {
		return c.retry(ctx, event, retryConf, result)
	}
	return c.markFailed(ctx, event, result, resultError(result))
}

func (c *WebhookDeliveryConsumer) markSuccess(ctx context.Context, event *entity.WebhookDeliveryMessage, statusCode int) error {
	now := time.Now()
	delivery := &entity.WebhookDelivery{
		DeliveryID:   event.DeliveryID,
		Status:       entity.DeliveryStatusSuccess,
		AttemptCount: event.Attempt + 1,
		LastSentAt:   &now,
		NextRetryAt:  nil,
		ResponseCode: &statusCode,
		ErrorMessage: "",
	}
	if event.Attempt == 0 {
		delivery.FirstSentAt = &now
	}
	return c.deliveryRepo.Update(ctx, delivery)
}

func (c *WebhookDeliveryConsumer) enrichWebhookExperiment(ctx context.Context, spaceID int64, expt *entity.Experiment) {
	if expt == nil {
		return
	}
	if expt.Stats == nil && c.resultSvc != nil {
		stats, err := c.resultSvc.GetStats(ctx, expt.ID, spaceID, &entity.Session{})
		if err != nil {
			logs.CtxWarn(ctx, "[Webhook] get expt stats fail, expt_id: %v, err: %v", expt.ID, err)
		} else {
			expt.Stats = stats
		}
	}
	if expt.AggregateResult == nil && c.aggrResultSvc != nil {
		results, err := c.aggrResultSvc.BatchGetExptAggrResultByExperimentIDs(ctx, spaceID, []int64{expt.ID})
		if err != nil {
			logs.CtxWarn(ctx, "[Webhook] get expt aggregate result fail, expt_id: %v, err: %v", expt.ID, err)
		} else if len(results) > 0 {
			expt.AggregateResult = results[0]
		}
	}
}

func (c *WebhookDeliveryConsumer) retry(ctx context.Context, event *entity.WebhookDeliveryMessage, retryConf *entity.WebhookRetryConf, result *componentwebhook.SendResult) error {
	now := time.Now()
	delay := retryDelay(retryConf, event.Attempt)
	nextRetryAt := now.Add(delay)
	statusCode := resultStatusCode(result)
	delivery := &entity.WebhookDelivery{
		DeliveryID:   event.DeliveryID,
		Status:       entity.DeliveryStatusRetrying,
		AttemptCount: event.Attempt + 1,
		LastSentAt:   &now,
		NextRetryAt:  &nextRetryAt,
		ResponseCode: statusCode,
		ErrorMessage: resultError(result),
	}
	if event.Attempt == 0 {
		delivery.FirstSentAt = &now
	}
	if err := c.deliveryRepo.Update(ctx, delivery); err != nil {
		return err
	}
	retryEvent := *event
	retryEvent.Attempt++
	return c.publisher.PublishWebhookDeliveryEvent(ctx, &retryEvent, &delay)
}

func (c *WebhookDeliveryConsumer) markFailed(ctx context.Context, event *entity.WebhookDeliveryMessage, result *componentwebhook.SendResult, errMsg string) error {
	now := time.Now()
	delivery := &entity.WebhookDelivery{
		DeliveryID:   event.DeliveryID,
		Status:       entity.DeliveryStatusFailed,
		AttemptCount: event.Attempt + 1,
		LastSentAt:   &now,
		NextRetryAt:  nil,
		ResponseCode: resultStatusCode(result),
		ErrorMessage: errMsg,
	}
	if event.Attempt == 0 {
		delivery.FirstSentAt = &now
	}
	return c.deliveryRepo.Update(ctx, delivery)
}

type WebhookDeliveryEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func NewWebhookDeliveryEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &WebhookDeliveryEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

func (e *WebhookDeliveryEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.WebhookDeliveryRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

func buildWebhookPayload(event *entity.WebhookDeliveryMessage, expt *entity.Experiment, webhookConf *entity.WebhookGlobalConf) *entity.WebhookPayload {
	return &entity.WebhookPayload{
		DeliveryID: event.DeliveryID,
		Event:      event.EventType,
		Timestamp:  time.Now().Unix(),
		Experiment: &entity.WebhookExptInfo{
			ID:        fmt.Sprintf("%d", expt.ID),
			Name:      expt.Name,
			Status:    webhookStatus(expt.Status),
			Progress:  buildWebhookProgress(expt.Stats),
			Metrics:   buildWebhookMetrics(event.EventType, expt.AggregateResult),
			ResultURL: webhookConf.BuildResultURL(event.SpaceID, expt.ID),
		},
	}
}

func webhookStatus(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Processing:
		return "processing"
	case entity.ExptStatus_Success:
		return "success"
	case entity.ExptStatus_Failed:
		return "failed"
	case entity.ExptStatus_Terminated:
		return "terminated"
	case entity.ExptStatus_SystemTerminated:
		return "terminated"
	default:
		return fmt.Sprintf("%d", status)
	}
}

func buildWebhookProgress(stats *entity.ExptStats) *entity.WebhookProgress {
	if stats == nil {
		return &entity.WebhookProgress{}
	}
	return &entity.WebhookProgress{
		Total:      int(stats.PendingItemCnt + stats.SuccessItemCnt + stats.FailItemCnt + stats.ProcessingItemCnt + stats.TerminatedItemCnt),
		Succeeded:  int(stats.SuccessItemCnt),
		Failed:     int(stats.FailItemCnt),
		Processing: int(stats.ProcessingItemCnt),
	}
}

func buildWebhookMetrics(eventType entity.WebhookEventType, aggrResult *entity.ExptAggregateResult) *entity.WebhookMetrics {
	if eventType != entity.WebhookEventSucceeded {
		return nil
	}
	metrics := &entity.WebhookMetrics{
		OverallScore:     buildWebhookScoreAgg(nil),
		EvaluatorMetrics: []*entity.WebhookEvaluatorAgg{},
	}
	if aggrResult == nil {
		return metrics
	}
	metrics.OverallScore = buildWebhookScoreAgg(aggrResult.WeightedResults)
	evaluatorIDs := make([]int64, 0, len(aggrResult.EvaluatorResults))
	for evaluatorID := range aggrResult.EvaluatorResults {
		evaluatorIDs = append(evaluatorIDs, evaluatorID)
	}
	sort.Slice(evaluatorIDs, func(i, j int) bool { return evaluatorIDs[i] < evaluatorIDs[j] })
	for _, evaluatorID := range evaluatorIDs {
		item := aggrResult.EvaluatorResults[evaluatorID]
		if item == nil {
			continue
		}
		name := ""
		if item.Name != nil {
			name = *item.Name
		}
		metrics.EvaluatorMetrics = append(metrics.EvaluatorMetrics, &entity.WebhookEvaluatorAgg{
			EvaluatorID:   fmt.Sprintf("%d", item.EvaluatorID),
			EvaluatorName: name,
			Score:         buildWebhookScoreAgg(item.AggregatorResults),
		})
	}
	return metrics
}

func buildWebhookScoreAgg(results []*entity.AggregatorResult) *entity.WebhookScoreAgg {
	score := &entity.WebhookScoreAgg{}
	for _, result := range results {
		if result == nil {
			continue
		}
		value := result.GetScore()
		switch result.AggregatorType {
		case entity.Average:
			score.Avg = &value
		case entity.Min:
			score.Min = &value
		case entity.Max:
			score.Max = &value
		}
	}
	return score
}

func retryDelay(conf *entity.WebhookRetryConf, attempt int) time.Duration {
	defaultConf := entity.DefaultWebhookRetryConf()
	if conf == nil || len(conf.RetryDelays) == 0 {
		conf = defaultConf
	}
	if attempt >= 0 && attempt < len(conf.RetryDelays) && conf.RetryDelays[attempt] > 0 {
		return conf.RetryDelays[attempt]
	}
	return defaultConf.RetryDelays[len(defaultConf.RetryDelays)-1]
}

func resultStatusCode(result *componentwebhook.SendResult) *int {
	if result == nil || result.StatusCode == 0 {
		return nil
	}
	return &result.StatusCode
}

func resultError(result *componentwebhook.SendResult) string {
	if result == nil {
		return "webhook sender returned nil result"
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	return fmt.Sprintf("webhook delivery failed, status_code: %d", result.StatusCode)
}
