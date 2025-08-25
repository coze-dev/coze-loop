package otel

type ResourceScopeSpan struct {
	Resource *Resource             `json:"resource,omitempty"`
	Scope    *InstrumentationScope `json:"scope,omitempty"`
	Span     *Span                 `json:"span,omitempty"`
}
