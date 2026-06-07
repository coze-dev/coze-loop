package entity

import (
	"encoding/json"
	"strconv"
	"strings"
)

type NotificationConf struct {
	Filter             *ExptListFilter         `json:"filter,omitempty"`
	Webhook            *WebhookConf            `json:"webhook,omitempty"`
	FeishuNotification *FeishuNotificationConf `json:"feishu_notification,omitempty"`
}

type WebhookConf struct {
	Enable bool   `json:"enable"`
	URLs   string `json:"urls,omitempty"`
	Secret string `json:"secret,omitempty"`
}

type FeishuNotificationConf struct {
	Enable bool `json:"enable"`
}

type WebhookEventType string

const (
	WebhookEventExperimentStarted    WebhookEventType = "experiment.started"
	WebhookEventExperimentSucceeded  WebhookEventType = "experiment.succeeded"
	WebhookEventExperimentFailed     WebhookEventType = "experiment.failed"
	WebhookEventExperimentTerminated WebhookEventType = "experiment.terminated"
)

type WebhookRetryEvent struct {
	DeliveryID  string `json:"delivery_id"`
	WebhookURL  string `json:"webhook_url"`
	RequestBody string `json:"request_body"`
	Secret      string `json:"secret"`
	AttemptNum  int    `json:"attempt_num"`
	SpaceID     int64  `json:"space_id"`
	ExptID      int64  `json:"expt_id"`
}

type WebhookPayload struct {
	DeliveryID   string                `json:"delivery_id"`
	CreateTime   string                `json:"create_time"`
	EventType    WebhookEventType      `json:"event_type"`
	ResourceType string                `json:"resource_type"`
	Summary      string                `json:"summary"`
	Data         WebhookExperimentData `json:"data"`
}

type WebhookExperimentData struct {
	ExperimentID   string                    `json:"experiment_id"`
	ExperimentName string                    `json:"experiment_name"`
	Status         string                    `json:"status"`
	Progress       *WebhookExperimentProgress `json:"progress"`
}

type WebhookExperimentProgress struct {
	Total     int64 `json:"total"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

func ExptStatusToWebhookEventType(status ExptStatus) (WebhookEventType, bool) {
	switch status {
	case ExptStatus_Processing:
		return WebhookEventExperimentStarted, true
	case ExptStatus_Success:
		return WebhookEventExperimentSucceeded, true
	case ExptStatus_Failed:
		return WebhookEventExperimentFailed, true
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return WebhookEventExperimentTerminated, true
	default:
		return "", false
	}
}

func ExptStatusToWebhookStatusString(status ExptStatus) string {
	switch status {
	case ExptStatus_Processing:
		return "started"
	case ExptStatus_Success:
		return "succeeded"
	case ExptStatus_Failed:
		return "failed"
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return "terminated"
	default:
		return "unknown"
	}
}

func (c *NotificationConf) ShouldNotify(status ExptStatus) bool {
	if c == nil || c.Filter == nil {
		return false
	}
	return c.matchFilter(status)
}

func (c *NotificationConf) ShouldWebhook(status ExptStatus) bool {
	if c == nil || c.Webhook == nil || !c.Webhook.Enable || c.Webhook.URLs == "" {
		return false
	}
	return c.ShouldNotify(status)
}

func (c *NotificationConf) ShouldFeishu(status ExptStatus) bool {
	if c == nil || c.FeishuNotification == nil || !c.FeishuNotification.Enable {
		return false
	}
	return c.ShouldNotify(status)
}

func (c *NotificationConf) GetWebhookURLs() []string {
	if c == nil || c.Webhook == nil || c.Webhook.URLs == "" {
		return nil
	}
	urls := strings.Split(c.Webhook.URLs, ",")
	result := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" {
			result = append(result, u)
		}
	}
	return result
}

func (c *NotificationConf) matchFilter(status ExptStatus) bool {
	if c.Filter == nil || len(c.Filter.Includes.Status) == 0 && len(c.Filter.Excludes.Status) == 0 {
		return true
	}
	statusVal := int64(status)
	if len(c.Filter.Includes.Status) > 0 {
		found := false
		for _, s := range c.Filter.Includes.Status {
			if s == statusVal {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(c.Filter.Excludes.Status) > 0 {
		for _, s := range c.Filter.Excludes.Status {
			if s == statusVal {
				return false
			}
		}
	}
	return true
}

func ParseNotificationConf(data []byte) *NotificationConf {
	if len(data) == 0 {
		return nil
	}
	var conf NotificationConf
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil
	}
	return &conf
}

func SerializeNotificationConf(conf *NotificationConf) []byte {
	if conf == nil {
		return nil
	}
	data, err := json.Marshal(conf)
	if err != nil {
		return nil
	}
	return data
}

func ParseFilterConditionStatusValues(valueStr string) []int64 {
	valueStr = strings.Trim(valueStr, "[]")
	if valueStr == "" {
		return nil
	}
	parts := strings.Split(valueStr, ",")
	result := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "\"")
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			result = append(result, v)
		}
	}
	return result
}
