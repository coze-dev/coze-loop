package expt

type ExptNotificationConf struct {
	Filter             *Filters                `thrift:"filter,1,optional" json:"filter,omitempty"`
	Webhook            *WebhookNotificationConf `thrift:"webhook,10,optional" json:"webhook,omitempty"`
	FeishuNotification *FeishuNotificationConf  `thrift:"feishu_notification,11,optional" json:"feishu_notification,omitempty"`
}

func NewExptNotificationConf() *ExptNotificationConf {
	return &ExptNotificationConf{}
}

func (p *ExptNotificationConf) InitDefault() {}

func (p *ExptNotificationConf) GetFilter() (v *Filters) {
	if p == nil || p.Filter == nil {
		return nil
	}
	return p.Filter
}

func (p *ExptNotificationConf) GetWebhook() (v *WebhookNotificationConf) {
	if p == nil || p.Webhook == nil {
		return nil
	}
	return p.Webhook
}

func (p *ExptNotificationConf) GetFeishuNotification() (v *FeishuNotificationConf) {
	if p == nil || p.FeishuNotification == nil {
		return nil
	}
	return p.FeishuNotification
}

func (p *ExptNotificationConf) IsSetFilter() bool {
	return p != nil && p.Filter != nil
}

func (p *ExptNotificationConf) IsSetWebhook() bool {
	return p != nil && p.Webhook != nil
}

func (p *ExptNotificationConf) IsSetFeishuNotification() bool {
	return p != nil && p.FeishuNotification != nil
}

type WebhookNotificationConf struct {
	Enable bool    `thrift:"enable,1,required" json:"enable,required"`
	Urls   *string `thrift:"urls,2,optional" json:"urls,omitempty"`
}

func NewWebhookNotificationConf() *WebhookNotificationConf {
	return &WebhookNotificationConf{}
}

func (p *WebhookNotificationConf) InitDefault() {}

func (p *WebhookNotificationConf) GetEnable() (v bool) {
	if p != nil {
		return p.Enable
	}
	return
}

func (p *WebhookNotificationConf) GetUrls() (v string) {
	if p == nil || p.Urls == nil {
		return ""
	}
	return *p.Urls
}

func (p *WebhookNotificationConf) IsSetUrls() bool {
	return p != nil && p.Urls != nil
}

type FeishuNotificationConf struct {
	Enable bool    `thrift:"enable,1,required" json:"enable,required"`
	UserID *string `thrift:"user_id,2,optional" json:"user_id,omitempty"`
}

func NewFeishuNotificationConf() *FeishuNotificationConf {
	return &FeishuNotificationConf{}
}

func (p *FeishuNotificationConf) InitDefault() {}

func (p *FeishuNotificationConf) GetEnable() (v bool) {
	if p != nil {
		return p.Enable
	}
	return
}

func (p *FeishuNotificationConf) GetUserID() (v string) {
	if p == nil || p.UserID == nil {
		return ""
	}
	return *p.UserID
}

func (p *FeishuNotificationConf) IsSetUserID() bool {
	return p != nil && p.UserID != nil
}
