// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// --- fakes -----------------------------------------------------------------

type fakeSender struct {
	statuses []int
	errs     []error
	calls    int
	recorded []entity.WebhookDelivery // deep-ish copies for assertion
}

func (f *fakeSender) Send(_ context.Context, d *entity.WebhookDelivery) (int, error) {
	i := f.calls
	f.calls++
	if d != nil {
		f.recorded = append(f.recorded, *d)
	}
	if i >= len(f.statuses) {
		return 0, errors.New("fake sender exhausted")
	}
	return f.statuses[i], f.errs[i]
}

type fakeDeliveryRepo struct {
	get      *entity.WebhookDelivery
	getErr   error
	updates  []*entity.WebhookDelivery
	updateFn func(*entity.WebhookDelivery) error
}

func (f *fakeDeliveryRepo) Create(_ context.Context, _ *entity.WebhookDelivery, _ ...db.Option) error {
	return nil
}

func (f *fakeDeliveryRepo) Update(_ context.Context, d *entity.WebhookDelivery, _ ...db.Option) error {
	copied := *d
	f.updates = append(f.updates, &copied)
	if f.updateFn != nil {
		return f.updateFn(d)
	}
	// mutate the tracked "current row" so subsequent GetByDeliveryID sees it —
	// mimics DB single-row semantics per delivery_id (idempotency contract).
	if f.get != nil && f.get.DeliveryID == d.DeliveryID {
		snapshot := *d
		f.get = &snapshot
	}
	return nil
}

func (f *fakeDeliveryRepo) GetByDeliveryID(_ context.Context, id string, _ ...db.Option) (*entity.WebhookDelivery, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.get == nil {
		return nil, nil
	}
	if f.get.DeliveryID != id {
		return nil, nil
	}
	c := *f.get
	return &c, nil
}

func (f *fakeDeliveryRepo) ListByExptID(_ context.Context, _ repo.ListDeliveryParams, _ ...db.Option) ([]*entity.WebhookDelivery, int64, error) {
	return nil, 0, nil
}

func (f *fakeDeliveryRepo) ListRetryable(_ context.Context, _ repo.ListRetryableParams, _ ...db.Option) ([]*entity.WebhookDelivery, error) {
	return nil, nil
}

type fakePublisher struct {
	published []*events.WebhookDeliveryEvent
	delayed   []time.Duration
	delayErr  error
}

func (f *fakePublisher) Publish(_ context.Context, evt *events.WebhookDeliveryEvent) error {
	f.published = append(f.published, evt)
	return nil
}

func (f *fakePublisher) PublishDelay(_ context.Context, evt *events.WebhookDeliveryEvent, delay time.Duration) error {
	f.published = append(f.published, evt)
	f.delayed = append(f.delayed, delay)
	return f.delayErr
}

type fakeConfiger struct {
	global *entity.WebhookGlobalConf
	retry  *entity.WebhookRetryConf
}

func (f fakeConfiger) GetWebhookConf(_ context.Context) *entity.WebhookGlobalConf {
	if f.global != nil {
		return f.global
	}
	return entity.DefaultWebhookGlobalConf()
}

func (f fakeConfiger) GetWebhookRetryConf(_ context.Context) *entity.WebhookRetryConf {
	if f.retry != nil {
		return f.retry
	}
	return entity.DefaultWebhookRetryConf()
}

func (f fakeConfiger) GetWebhookRateLimitConf(_ context.Context) *entity.WebhookRateLimitConf {
	return entity.DefaultWebhookRateLimitConf()
}

func (f fakeConfiger) GetWebhookURLLimitConf(_ context.Context) *entity.WebhookURLLimitConf {
	return entity.DefaultWebhookURLLimitConf()
}

func (f fakeConfiger) GetWebhookSecurityConf(_ context.Context) *entity.WebhookSecurityConf {
	return entity.DefaultWebhookSecurityConf()
}

// --- helpers ---------------------------------------------------------------

func newTestConsumer(sender *fakeSender, repo *fakeDeliveryRepo, pub *fakePublisher, cfg fakeConfiger) *WebhookDeliveryConsumer {
	fixed := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	return &WebhookDeliveryConsumer{
		sender:       sender,
		deliveryRepo: repo,
		publisher:    pub,
		configer:     cfg,
		now:          func() time.Time { return fixed },
	}
}

func mkPendingDelivery(id string) *entity.WebhookDelivery {
	return &entity.WebhookDelivery{
		DeliveryID:   id,
		SpaceID:      42,
		ExperimentID: 100,
		Event:        entity.WebhookDeliveryEventSucceeded,
		URL:          "https://stub.local/cb",
		Payload:      []byte(`{"delivery_id":"` + id + `"}`),
		Status:       entity.WebhookDeliveryStatusPending,
	}
}

// --- tests -----------------------------------------------------------------

// TestConsumerProcess_SuccessClearsLastError verifies R2 2xx path: status
// flips to `succeeded`, last_error / last_response_code are reset (relies on
// dao Select-whitelist Update honouring zero values), no re-enqueue happens.
// Aligned with N-C-01 / N-C-02 first-attempt success and covers the retry-success
// path's "clear last_error" expectation (B-B-01 counter-case).
func TestConsumerProcess_SuccessClearsLastError(t *testing.T) {
	sender := &fakeSender{statuses: []int{200}, errs: []error{nil}}
	priorErr := "boom"
	d := mkPendingDelivery("N-C-01")
	d.AttemptCount = 1
	d.Status = entity.WebhookDeliveryStatusRetrying
	d.LastError = priorErr
	d.LastResponseCode = 500
	repo := &fakeDeliveryRepo{get: d}
	pub := &fakePublisher{}
	c := newTestConsumer(sender, repo, pub, fakeConfiger{})

	require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: d.DeliveryID}))
	require.Len(t, repo.updates, 1)
	upd := repo.updates[0]
	require.Equal(t, entity.WebhookDeliveryStatusSucceeded, upd.Status)
	require.Equal(t, "", upd.LastError, "success must clear last_error (whitelist Update)")
	require.Equal(t, 200, upd.LastResponseCode)
	require.Equal(t, 2, upd.AttemptCount)
	require.Empty(t, pub.delayed, "success path never re-enqueues")
}

// TestConsumerProcess_Non2xxEnqueuesNextBackoff covers E-B-01 / E-B-02: a
// non-2xx response transitions status to retrying and enqueues the next
// attempt at the correct backoff level (60/300/1800 → attempt 1/2/3).
func TestConsumerProcess_Non2xxEnqueuesNextBackoff(t *testing.T) {
	cases := []struct {
		attemptBefore int
		wantDelay     time.Duration
	}{
		{1, 60 * time.Second},
		{2, 300 * time.Second},
		{3, 1800 * time.Second},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("attempt=%d", tc.attemptBefore), func(t *testing.T) {
			sender := &fakeSender{statuses: []int{500}, errs: []error{errors.New("upstream 500")}}
			d := mkPendingDelivery("E-B-01")
			d.AttemptCount = tc.attemptBefore - 1
			if tc.attemptBefore > 1 {
				d.Status = entity.WebhookDeliveryStatusRetrying
			}
			repo := &fakeDeliveryRepo{get: d}
			pub := &fakePublisher{}
			c := newTestConsumer(sender, repo, pub, fakeConfiger{})

			require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: d.DeliveryID}))
			require.Len(t, repo.updates, 1)
			upd := repo.updates[0]
			require.Equal(t, entity.WebhookDeliveryStatusRetrying, upd.Status)
			require.Equal(t, tc.attemptBefore, upd.AttemptCount)
			require.Equal(t, 500, upd.LastResponseCode)
			require.Contains(t, upd.LastError, "upstream 500")
			require.Len(t, pub.delayed, 1)
			require.Equal(t, tc.wantDelay, pub.delayed[0])
			require.Equal(t, tc.attemptBefore+1, pub.published[0].Attempt)
		})
	}
}

// TestConsumerProcess_MaxAttemptsFinalFailed covers B-C-01: after
// max_attempts total tries the row is marked `final_failed` and no further
// re-enqueue happens (bounded work).
func TestConsumerProcess_MaxAttemptsFinalFailed(t *testing.T) {
	sender := &fakeSender{statuses: []int{500}, errs: []error{errors.New("upstream 500")}}
	d := mkPendingDelivery("B-C-01")
	d.AttemptCount = 3 // this attempt will bump to 4 = max_attempts
	d.Status = entity.WebhookDeliveryStatusRetrying
	repo := &fakeDeliveryRepo{get: d}
	pub := &fakePublisher{}
	c := newTestConsumer(sender, repo, pub, fakeConfiger{})

	require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: d.DeliveryID}))
	require.Len(t, repo.updates, 1)
	upd := repo.updates[0]
	require.Equal(t, entity.WebhookDeliveryStatusFinalFailed, upd.Status)
	require.Equal(t, 4, upd.AttemptCount)
	require.Empty(t, pub.delayed, "final_failed must not re-enqueue")
}

// TestConsumerProcess_TransportErrorStillEnqueues ensures a transport / DNS
// failure (statusCode=0) is treated like a 5xx and progresses the retry
// state — covers B-B-01 (timeout) and E-B-03 (DNS/connect fail).
func TestConsumerProcess_TransportErrorStillEnqueues(t *testing.T) {
	sender := &fakeSender{statuses: []int{0}, errs: []error{errors.New("dial tcp: i/o timeout")}}
	d := mkPendingDelivery("B-B-01")
	repo := &fakeDeliveryRepo{get: d}
	pub := &fakePublisher{}
	c := newTestConsumer(sender, repo, pub, fakeConfiger{})

	require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: d.DeliveryID}))
	require.Len(t, repo.updates, 1)
	upd := repo.updates[0]
	require.Equal(t, entity.WebhookDeliveryStatusRetrying, upd.Status)
	require.Contains(t, upd.LastError, "i/o timeout")
	require.Equal(t, 0, upd.LastResponseCode)
	require.Len(t, pub.delayed, 1)
	require.Equal(t, 60*time.Second, pub.delayed[0])
}

// TestConsumerProcess_SkipsTerminalDelivery covers R5 idempotency: consumer
// re-delivery of a message for an already-terminal row (succeeded /
// final_failed / dry_run) must be a no-op — no sender call, no repo update,
// no re-enqueue.
func TestConsumerProcess_SkipsTerminalDelivery(t *testing.T) {
	terminals := []string{
		entity.WebhookDeliveryStatusSucceeded,
		entity.WebhookDeliveryStatusFinalFailed,
		entity.WebhookDeliveryStatusDryRun,
		entity.WebhookDeliveryStatusRateLimited,
	}
	for _, st := range terminals {
		t.Run(st, func(t *testing.T) {
			sender := &fakeSender{}
			d := mkPendingDelivery("R5-idem")
			d.Status = st
			d.AttemptCount = 2
			repo := &fakeDeliveryRepo{get: d}
			pub := &fakePublisher{}
			c := newTestConsumer(sender, repo, pub, fakeConfiger{})

			require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: d.DeliveryID}))
			require.Equal(t, 0, sender.calls)
			require.Empty(t, repo.updates)
			require.Empty(t, pub.delayed)
		})
	}
}

// TestConsumerProcess_GlobalDisabledSkips exercises E-I-01: when
// WebhookGlobalConf.Enabled == false the consumer path never touches the
// sender or repo — the kill switch is respected end-to-end.
func TestConsumerProcess_GlobalDisabledSkips(t *testing.T) {
	sender := &fakeSender{}
	repo := &fakeDeliveryRepo{get: mkPendingDelivery("kill")}
	pub := &fakePublisher{}
	cfg := fakeConfiger{global: &entity.WebhookGlobalConf{Enabled: false}}
	c := newTestConsumer(sender, repo, pub, cfg)

	require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: "kill"}))
	require.Equal(t, 0, sender.calls)
	require.Empty(t, repo.updates)
	require.Empty(t, pub.delayed)
}

// TestConsumerProcess_CustomMaxAttemptsOverride verifies E-C-01: configer
// overrides the default schedule; a max_attempts=2 config finalises the row
// after 2 tries rather than 4.
func TestConsumerProcess_CustomMaxAttemptsOverride(t *testing.T) {
	sender := &fakeSender{statuses: []int{500}, errs: []error{errors.New("upstream 500")}}
	d := mkPendingDelivery("E-C-01")
	d.AttemptCount = 1
	d.Status = entity.WebhookDeliveryStatusRetrying
	repo := &fakeDeliveryRepo{get: d}
	pub := &fakePublisher{}
	cfg := fakeConfiger{retry: &entity.WebhookRetryConf{BackoffSec: []int{60}, MaxAttempts: 2, RequestTimeoutMS: 5000}}
	c := newTestConsumer(sender, repo, pub, cfg)

	require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: d.DeliveryID}))
	require.Len(t, repo.updates, 1)
	require.Equal(t, entity.WebhookDeliveryStatusFinalFailed, repo.updates[0].Status)
	require.Empty(t, pub.delayed)
}

// TestConsumerProcess_MissingRowIsNoop exercises the dispatcher-out-of-sync
// path: the consumer must not error out when the row is gone — MQ delivery
// is at-least-once, so it may see events for rows the operator has purged.
func TestConsumerProcess_MissingRowIsNoop(t *testing.T) {
	sender := &fakeSender{}
	repo := &fakeDeliveryRepo{}
	pub := &fakePublisher{}
	c := newTestConsumer(sender, repo, pub, fakeConfiger{})

	require.NoError(t, c.Process(context.Background(), &events.WebhookDeliveryEvent{DeliveryID: "ghost"}))
	require.Equal(t, 0, sender.calls)
	require.Empty(t, repo.updates)
}

// TestNextBackoff_ExhaustedReusesLastEntry pins the backoff floor when
// operators trim the schedule to <max_attempts entries. This guards against
// off-by-one when computing the last retry's delay.
func TestNextBackoff_ExhaustedReusesLastEntry(t *testing.T) {
	seq := []int{60, 300, 1800}
	require.Equal(t, 60*time.Second, nextBackoff(seq, 1))
	require.Equal(t, 300*time.Second, nextBackoff(seq, 2))
	require.Equal(t, 1800*time.Second, nextBackoff(seq, 3))
	require.Equal(t, 1800*time.Second, nextBackoff(seq, 4), "exhausted schedule reuses last entry")
	require.Equal(t, 60*time.Second, nextBackoff(nil, 3), "empty schedule falls back to 60s")
}
