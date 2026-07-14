// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// Sub-cases align with case_manifest entries the T4 layer is meant to
// short-circuit (E-A-02 / E-E-01..03 / B-E-01 / B-E-02) plus N-L-01 (nil conf
// = old-client compat is a no-op, not an error).
func TestValidateNotificationConf(t *testing.T) {
	tenURLs := makeURLs(10)
	elevenURLs := makeURLs(11)

	tests := []struct {
		name           string
		conf           *domain_expt.ExptNotificationConf
		wantErr        bool
		wantMsgFragment string
	}{
		{
			name:    "nil conf: old-client compat (N-L-01)",
			conf:    nil,
			wantErr: false,
		},
		{
			name: "webhook nil is a no-op",
			conf: &domain_expt.ExptNotificationConf{},
			wantErr: false,
		},
		{
			name: "exactly 10 urls: boundary allowed (B-E-01)",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(true),
					Urls:   ptr.Of(tenURLs),
				},
			},
			wantErr: false,
		},
		{
			name: "11 urls: over-cap rejected (B-E-02)",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(true),
					Urls:   ptr.Of(elevenURLs),
				},
			},
			wantErr:         true,
			wantMsgFragment: "max_urls_per_experiment",
		},
		{
			name: "http scheme rejected (E-E-01)",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(true),
					Urls:   ptr.Of("http://a.com/cb"),
				},
			},
			wantErr:         true,
			wantMsgFragment: "scheme/https",
		},
		{
			name: "enable=true with empty urls rejected (E-E-02)",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(true),
					Urls:   ptr.Of(""),
				},
			},
			wantErr:         true,
			wantMsgFragment: "requires non-empty urls",
		},
		{
			name: "url missing host rejected (E-E-03)",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(true),
					Urls:   ptr.Of("https:///cb"),
				},
			},
			wantErr:         true,
			wantMsgFragment: "webhook url missing host",
		},
		{
			name: "enable=false is allowed even with empty urls",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(false),
					Urls:   ptr.Of(""),
				},
			},
			wantErr: false,
		},
		{
			name: "filter with unsupported operator rejected (E-A-02)",
			conf: &domain_expt.ExptNotificationConf{
				Filter: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{Field: &domain_expt.FilterField{}, Operator: domain_expt.FilterOperatorType_Greater, Value: "1"},
					},
				},
			},
			wantErr:         true,
			wantMsgFragment: "notification filter operator unsupported",
		},
		{
			name: "filter with IN operator allowed",
			conf: &domain_expt.ExptNotificationConf{
				Filter: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{Field: &domain_expt.FilterField{}, Operator: domain_expt.FilterOperatorType_In, Value: "success"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "filter with NotIn operator allowed",
			conf: &domain_expt.ExptNotificationConf{
				Filter: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{Field: &domain_expt.FilterField{}, Operator: domain_expt.FilterOperatorType_NotIn, Value: "failed"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "3 urls: below cap allowed (N-E-01)",
			conf: &domain_expt.ExptNotificationConf{
				Webhook: &domain_expt.WebhookNotificationConf{
					Enable: ptr.Of(true),
					Urls:   ptr.Of("https://a.com/cb,https://b.com/cb,https://c.com/cb"),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNotificationConf(tt.conf)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantMsgFragment != "" {
					require.Contains(t, err.Error(), tt.wantMsgFragment)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSplitWebhookURLs(t *testing.T) {
	require.Nil(t, splitWebhookURLs(""))
	require.Equal(t, []string{"https://a.com"}, splitWebhookURLs("https://a.com"))
	// empty tokens are dropped so trailing commas don't inflate the count
	require.Equal(t, []string{"https://a.com", "https://b.com"}, splitWebhookURLs("https://a.com,,https://b.com,"))
	require.Equal(t, []string{"https://a.com", "https://b.com"}, splitWebhookURLs("  https://a.com  ,  https://b.com  "))
}

// makeURLs builds a comma-separated blob of N distinct https URLs so tests
// can drive `WebhookNotificationConf.Urls` at exact boundaries.
func makeURLs(n int) string {
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = "https://example.com/cb/" + string(rune('a'+i%26))
	}
	return strings.Join(parts, ",")
}
