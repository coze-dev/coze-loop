// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"fmt"
	"net/url"
	"strings"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// notification_conf validation constants — kept aligned with
// `entity.WebhookURLLimitConf` defaults so front-end / CLI / OpenAPI hit the
// same boundaries. Values here are the hard ceiling; the runtime
// `IWebhookConfiger.GetWebhookURLLimitConf` may narrow them per-space but not
// widen (T4 IDL layer is the outer perimeter).
const (
	maxURLsPerExperiment = 10
	webhookURLScheme     = "https"
)

// validateNotificationConf enforces the T4 rules on the inbound IDL config:
//   - conf == nil short-circuits (old client compat: N-L-01/N-L-02).
//   - webhook.urls comma-separated cap ≤ maxURLsPerExperiment (B-E-01/B-E-02).
//   - each URL parseable + scheme=https (E-E-01/E-E-03).
//   - webhook.enable=true implies non-empty urls (E-E-02).
//   - filter.filter_conditions[].operator must be a supported In/NotIn/Unknown
//     value (E-A-02); anything else is rejected before it can reach domain.
//
// Returns 400 (`errno.CommonInvalidParamCode`) with a stable `extra_msg` that
// verify/e2e can pattern-match on.
func validateNotificationConf(conf *domain_expt.ExptNotificationConf) error {
	if conf == nil {
		return nil
	}
	if err := validateWebhookConf(conf.GetWebhook()); err != nil {
		return err
	}
	if err := validateNotificationFilter(conf.GetFilter()); err != nil {
		return err
	}
	return nil
}

func validateWebhookConf(w *domain_expt.WebhookNotificationConf) error {
	if w == nil {
		return nil
	}
	urls := splitWebhookURLs(w.GetUrls())
	if len(urls) > maxURLsPerExperiment {
		return errorx.NewByCode(errno.CommonInvalidParamCode,
			errorx.WithExtraMsg(fmt.Sprintf("max_urls_per_experiment exceeded: got %d, max %d", len(urls), maxURLsPerExperiment)))
	}
	if w.GetEnable() && len(urls) == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode,
			errorx.WithExtraMsg("webhook enable=true requires non-empty urls"))
	}
	for _, raw := range urls {
		if err := validateWebhookURL(raw); err != nil {
			return err
		}
	}
	return nil
}

// splitWebhookURLs cracks the comma-separated `urls` blob the IDL still
// carries as `optional string` (see residual_risks note in iter_5). Trimmed
// tokens are what feeds the URL cap + scheme checks; empty tokens are dropped
// so `"https://a.com,,https://b.com"` doesn't inflate the count.
func splitWebhookURLs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode,
			errorx.WithExtraMsg(fmt.Sprintf("webhook url invalid format: %q", raw)))
	}
	if u.Host == "" {
		return errorx.NewByCode(errno.CommonInvalidParamCode,
			errorx.WithExtraMsg(fmt.Sprintf("webhook url missing host: %q", raw)))
	}
	if !strings.EqualFold(u.Scheme, webhookURLScheme) {
		return errorx.NewByCode(errno.CommonInvalidParamCode,
			errorx.WithExtraMsg(fmt.Sprintf("webhook url scheme/https required, got %q", u.Scheme)))
	}
	return nil
}

// validateNotificationFilter permits the same subset the dispatcher's
// `filterMatch` actually reads (In/NotIn). `Unknown` is tolerated so that old
// clients not carrying the enum survive (N-L-01 compat), matching how the
// dispatcher treats it as "no filter". Any other operator is rejected here so
// verify can rely on a 400 before the request reaches domain.
func validateNotificationFilter(f *domain_expt.Filters) error {
	if f == nil {
		return nil
	}
	for _, c := range f.GetFilterConditions() {
		if c == nil {
			continue
		}
		switch c.GetOperator() {
		case domain_expt.FilterOperatorType_Unknown,
			domain_expt.FilterOperatorType_In,
			domain_expt.FilterOperatorType_NotIn:
			// supported
		default:
			return errorx.NewByCode(errno.CommonInvalidParamCode,
				errorx.WithExtraMsg(fmt.Sprintf("notification filter operator unsupported: %v", c.GetOperator())))
		}
	}
	return nil
}
