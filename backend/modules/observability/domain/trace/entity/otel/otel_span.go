// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package otel

type ResourceScopeSpan struct {
	Resource *Resource             `json:"resource,omitempty"`
	Scope    *InstrumentationScope `json:"scope,omitempty"`
	Span     *Span                 `json:"span,omitempty"`
}