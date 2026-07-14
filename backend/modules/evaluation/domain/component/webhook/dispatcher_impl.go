// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NewWebhookDispatcher wires the dispatcher with the exact 3-arg signature
// that `cozeloop-commercial/modules/evaluation/application/wire_gen.go`
// already references — sender is not injected here because the actual HTTP
// send happens in the MQ consumer path (dispatcher only persists + enqueues).
func NewWebhookDispatcher(
	deliveryRepo repo.IWebhookDeliveryRepo,
	publisher events.WebhookDeliveryEventPublisher,
	configer component.IWebhookConfiger,
) IWebhookDispatcher {
	return &webhookDispatcher{
		deliveryRepo: deliveryRepo,
		publisher:    publisher,
		configer:     configer,
		newDeliveryID: func() string {
			return uuid.NewString()
		},
	}
}

type webhookDispatcher struct {
	deliveryRepo  repo.IWebhookDeliveryRepo
	publisher     events.WebhookDeliveryEventPublisher
	configer      component.IWebhookConfiger
	newDeliveryID func() string
}

// Dispatch fans out the event according to NotifyConf + InternalRules. It
// respects the global gate and dry_run flag, evaluates the filter, then for
// each accepted webhook URL creates a `webhook_delivery` row and publishes the
// first delivery event. Errors from a single URL never abort the whole fan
// out — every URL is best-effort logged.
func (d *webhookDispatcher) Dispatch(ctx context.Context, req *DispatchRequest) error {
	if req == nil || req.Experiment == nil {
		return nil
	}

	global := d.configer.GetWebhookConf(ctx)
	if global != nil && !global.Enabled {
		return nil
	}

	rules := buildRules(req.NotifyConf, req.InternalRules)
	if len(rules) == 0 {
		return nil
	}

	dryRun := global != nil && global.DryRun
	for i := range rules {
		rule := rules[i]
		if !filterMatch(rule.Filter, req.Event) {
			continue
		}
		if rule.Webhook == nil || !rule.Webhook.Enable {
			continue
		}
		for _, url := range rule.Webhook.URLs {
			if url == "" {
				continue
			}
			d.dispatchOne(ctx, req, url, rule.InternalSource, dryRun)
		}
	}
	return nil
}

func (d *webhookDispatcher) dispatchOne(ctx context.Context, req *DispatchRequest, url, internalSource string, dryRun bool) {
	deliveryID := d.newDeliveryID()
	payload := buildPayload(deliveryID, req)
	body, _ := json.Marshal(payload)

	now := time.Now()
	initialStatus := entity.WebhookDeliveryStatusPending
	if dryRun {
		// dry_run keeps an audit row but never enqueues; use a dedicated
		// status so consumers / observers can distinguish a real pending
		// delivery from a dry-run one (E-I-02 assertion).
		initialStatus = entity.WebhookDeliveryStatusDryRun
	}

	delivery := &entity.WebhookDelivery{
		DeliveryID:     deliveryID,
		SpaceID:        req.SpaceID,
		ExperimentID:   req.Experiment.ID,
		Event:          req.Event,
		URL:            url,
		Payload:        body,
		Status:         initialStatus,
		AttemptCount:   0,
		InternalSource: internalSource,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := d.deliveryRepo.Create(ctx, delivery); err != nil {
		logs.CtxWarn(ctx, "webhook delivery create failed, expt=%d event=%s url=%s err=%v",
			req.Experiment.ID, req.Event, url, err)
		return
	}
	if dryRun {
		return
	}
	evt := &events.WebhookDeliveryEvent{
		DeliveryID:   deliveryID,
		SpaceID:      req.SpaceID,
		ExperimentID: req.Experiment.ID,
		Event:        req.Event,
		Attempt:      1,
	}
	if err := d.publisher.Publish(ctx, evt); err != nil {
		logs.CtxWarn(ctx, "webhook delivery enqueue failed, delivery_id=%s err=%v", deliveryID, err)
	}
}

func buildRules(conf *entity.ExptNotificationConf, internalRules []entity.ExptNotificationRule) []entity.ExptNotificationRule {
	out := make([]entity.ExptNotificationRule, 0, len(internalRules)+1)
	if conf != nil {
		out = append(out, entity.ExptNotificationRule{
			Filter:  conf.Filter,
			Webhook: conf.Webhook,
			Feishu:  conf.FeishuNotification,
		})
	}
	for i := range internalRules {
		r := internalRules[i]
		if r.InternalSource == "" {
			r.InternalSource = entity.WebhookInternalSourceBITs
		}
		out = append(out, r)
	}
	return out
}

// filterMatch evaluates one filter against the incoming event string.
// Empty / nil filter matches everything (default-notify behaviour); empty
// `Values` never matches to avoid accidental all-through routing.
func filterMatch(filter *entity.ExptNotificationFilter, event string) bool {
	if filter == nil {
		return true
	}
	if filter.Field != entity.ExptNotificationFieldTypeExptStatus {
		return true
	}
	if len(filter.Values) == 0 {
		return false
	}
	present := false
	for _, v := range filter.Values {
		if v == event || v == entity.EventToStatusAlias(event) {
			present = true
			break
		}
	}
	switch filter.Operator {
	case entity.ExptNotificationOperatorNOTIN:
		return !present
	default:
		return present
	}
}

// buildPayload assembles the JSON body per §Webhook Payload contract:
// delivery_id / event / timestamp / experiment.{id,name,status,progress}.
func buildPayload(deliveryID string, req *DispatchRequest) map[string]any {
	expt := req.Experiment
	progress := map[string]any{
		"total":     0,
		"succeeded": 0,
		"failed":    0,
	}
	if expt != nil && expt.Stats != nil {
		total := expt.Stats.PendingItemCnt + expt.Stats.ProcessingItemCnt + expt.Stats.SuccessItemCnt + expt.Stats.FailItemCnt
		progress["total"] = int(total)
		progress["succeeded"] = int(expt.Stats.SuccessItemCnt)
		progress["failed"] = int(expt.Stats.FailItemCnt)
	}
	exptPayload := map[string]any{
		"progress": progress,
	}
	if expt != nil {
		exptPayload["id"] = fmt.Sprintf("%d", expt.ID)
		exptPayload["name"] = expt.Name
		exptPayload["status"] = int32(expt.Status)
	}
	return map[string]any{
		"delivery_id": deliveryID,
		"event":       req.Event,
		"timestamp":   time.Now().Unix(),
		"experiment":  exptPayload,
	}
}
