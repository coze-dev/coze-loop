// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

type ResourceScopeSpan struct {
	Resource *Resource             `json:"resource,omitempty"`
	Scope    *InstrumentationScope `json:"scope,omitempty"`
	Span     *Span                 `json:"span,omitempty"`
}
