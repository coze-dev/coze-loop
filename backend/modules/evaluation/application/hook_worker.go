// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

var (
	ErrHookWorkerConfiguration = errors.New("invalid hook worker configuration")
	ErrHookWorkerRunning       = errors.New("hook worker already running")
	ErrHookWorkerStopped       = errors.New("hook worker stopped")
	ErrHookWorkerScope         = errors.New("hook worker scope mismatch")
)

const (
	hookWorkerDependencyTimeout = 5 * time.Second
	hookWorkerFallbackInterval  = 10 * time.Second
	hookWorkerEffectAttempts    = 3
	hookWorkerPendingPerClass   = 100
	hookWorkerPendingPerSpace   = 2
)

type HookWorkerDependencies struct {
	// Supplied once by trusted deployment wiring, never a message or user context.
	ExecutionScope, Owner string
	ScanRepo              repo.IHookScanRepo
	Clock                 hook.WorkerClock
	Config                hook.RuntimeConfigProvider
	Executor              hook.AttemptExecutor
	IDs                   idgen.IIDGenerator
	Coordinator           hook.WorkerCoordinator
	Observer              hook.WorkerObserver
}

// HookWorker owns only a process-local pool. Construction starts no goroutines.
// Run blocks until its context is cancelled or Stop joins that Run generation.
type HookWorker struct {
	deps    HookWorkerDependencies
	mu      sync.Mutex
	session *hookWorkerSession
}

var _ hook.WakeHandler = (*HookWorker)(nil)

type hookWorkerSession struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wake   chan struct{}
	jobs   sync.WaitGroup
	// Protected by HookWorker.mu; no queue of work survives a Run generation.
	active map[string]int64
	spaces map[int64]int
	// Owned only by the scan loop, never by execution goroutines.
	streams      [5]hookWorkerStream
	first        int
	pending      [5][]hookWorkerTask
	overflow     [5]*hookWorkerOverflow
	queued       map[string]bool
	nextDispatch int
}

type hookWorkerStream struct {
	kind   entity.HookScanKind
	status entity.HookOperationStatus
	now    time.Time
	cursor *entity.HookScanCursor
}

type hookWorkerOverflow struct {
	stream hookWorkerStream
	page   []hookWorkerTask
	tail   bool
}

func newHookWorkerSession(ctx context.Context) *hookWorkerSession {
	ctx, cancel := context.WithCancel(ctx)
	return &hookWorkerSession{ctx: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1), active: make(map[string]int64), spaces: make(map[int64]int), queued: make(map[string]bool), streams: [5]hookWorkerStream{
		{kind: entity.HookScanDue, status: entity.HookOperationPending},
		{kind: entity.HookScanDue, status: entity.HookOperationRetryWait},
		{kind: entity.HookScanExpired}, {kind: entity.HookScanPreparing}, {kind: entity.HookScanFinalize},
	}}
}

func NewHookWorker(d HookWorkerDependencies) (*HookWorker, error) {
	for _, dep := range []any{d.ScanRepo, d.Clock, d.Config, d.Executor, d.IDs, d.Coordinator, d.Observer} {
		if hookWorkerNil(dep) {
			return nil, ErrHookWorkerConfiguration
		}
	}
	for _, text := range []string{d.ExecutionScope, d.Owner} {
		if len(text) == 0 || len(text) > 128 {
			return nil, ErrHookWorkerConfiguration
		}
		for _, b := range []byte(text) {
			if b < 33 || b > 126 {
				return nil, ErrHookWorkerConfiguration
			}
		}
	}
	return &HookWorker{deps: d}, nil
}

func hookWorkerNil(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}

func (w *HookWorker) Run(ctx context.Context) error {
	if w == nil || ctx == nil {
		return ErrHookWorkerConfiguration
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	w.mu.Lock()
	if w.session != nil {
		w.mu.Unlock()
		return ErrHookWorkerRunning
	}
	s := newHookWorkerSession(ctx)
	w.session = s
	w.mu.Unlock()
	defer func() {
		s.cancel()
		s.jobs.Wait()
		w.mu.Lock()
		w.session = nil
		close(s.done)
		w.mu.Unlock()
	}()
	for s.ctx.Err() == nil {
		// Coalesce hints received during the previous pacing window into this scan.
		select {
		case <-s.wake:
		default:
		}
		interval := w.runRound(s)
		wakeAfter := time.Now().Add(time.Second)
		timer := time.NewTimer(interval)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		case <-s.wake:
			timer.Stop()
			// A hint advances inspection, never permission or cursor position. Even
			// a continuous wake storm leaves a one-second floor between rounds.
			if delay := time.Until(wakeAfter); delay > 0 {
				pace := time.NewTimer(delay)
				select {
				case <-s.ctx.Done():
					pace.Stop()
					return nil
				case <-pace.C:
				}
			}
		}
	}
	return nil
}

// Stop cancels and joins the current generation; a later explicit Run may start
// a new generation. A timed-out Stop does not detach still-running dependencies.
func (w *HookWorker) Stop(ctx context.Context) error {
	if w == nil || ctx == nil {
		return ErrHookWorkerConfiguration
	}
	w.mu.Lock()
	s := w.session
	if s != nil {
		s.cancel()
	}
	w.mu.Unlock()
	if s == nil {
		return nil
	}
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *HookWorker) Wake(ctx context.Context, event entity.HookWakeEvent) error {
	if w == nil || ctx == nil {
		return ErrHookWorkerConfiguration
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, err := entity.EncodeHookWakeEvent(event); err != nil {
		return err
	}
	if event.ExecutionScope != w.deps.ExecutionScope {
		return ErrHookWorkerScope
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.session
	if s == nil || s.ctx.Err() != nil {
		return ErrHookWorkerStopped
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (w *HookWorker) observe(code hook.WorkerEventCode, kind entity.HookScanKind) {
	w.deps.Observer.Observe(hook.WorkerEvent{Code: code, Kind: kind})
}

func (w *HookWorker) config(ctx context.Context) (entity.HookRuntimeConfig, bool) {
	ctx, cancel := context.WithTimeout(ctx, hookWorkerDependencyTimeout)
	defer cancel()
	c, err := w.deps.Config.GetRuntimeConfig(ctx)
	if err != nil || ctx.Err() != nil || entity.ValidateHookWorkerConfig(c) != nil {
		w.observe(hook.WorkerConfigFailed, "")
		return entity.HookRuntimeConfig{}, false
	}
	return c, true
}

type hookWorkerTask struct {
	kind      entity.HookScanKind
	operation *entity.HookOperationCandidate
	run       entity.HookRunCandidate
}

func (w *HookWorker) runRound(s *hookWorkerSession) time.Duration {
	c, ok := w.config(s.ctx)
	if !ok {
		return hookWorkerFallbackInterval
	}
	interval := time.Duration(c.ScanIntervalSeconds) * time.Second
	if !c.WorkerEnabled {
		w.observe(hook.WorkerPaused, "")
		return interval
	}
	first := s.first
	s.first = (s.first + 1) % len(s.streams)
	for n := range s.streams {
		if s.ctx.Err() != nil {
			return interval
		}
		i := (first + n) % len(s.streams)
		page := w.scanPage(s, &s.streams[i], int(c.ScanBatchSize))
		w.resumeOverflow(s, i, int(c.ScanBatchSize))
		var deferred []hookWorkerTask
		for _, task := range page {
			if !w.enqueue(s, i, task) && s.overflow[i] == nil {
				if deferred == nil {
					deferred = make([]hookWorkerTask, 0, hookWorkerPendingPerClass)
				}
				deferred = append(deferred, task)
			}
		}
		if len(deferred) > 0 {
			stream := s.streams[i]
			if stream.cursor != nil {
				cursor := *stream.cursor
				stream.cursor = &cursor
			}
			s.overflow[i] = &hookWorkerOverflow{stream: stream, page: deferred, tail: stream.cursor == nil}
		}
	}
	w.dispatchPending(s, c)
	return interval
}

// Retain a rejected page until every candidate has a queue/execution opportunity.
// Its independent continuation covers later pages even as normal discovery wraps.
func (w *HookWorker) resumeOverflow(s *hookWorkerSession, class, limit int) {
	r := s.overflow[class]
	if r == nil || s.ctx.Err() != nil {
		return
	}
	if len(r.page) == 0 && !r.tail {
		r.page = append(r.page, w.scanPage(s, &r.stream, limit)...)
		r.tail = r.stream.cursor == nil
	}
	remaining := r.page[:0]
	for _, task := range r.page {
		if !w.enqueue(s, class, task) {
			remaining = append(remaining, task)
		}
	}
	for i := len(remaining); i < len(r.page); i++ {
		r.page[i] = hookWorkerTask{}
	}
	r.page = remaining
	if len(r.page) == 0 && r.tail {
		s.overflow[class] = nil
	}
}

// False means only capacity blocked the candidate; it needs overflow recovery.
func (w *HookWorker) enqueue(s *hookWorkerSession, class int, task hookWorkerTask) bool {
	key, id := task.identity()
	if key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID <= 0 || (task.operation != nil && (task.operation.OperationID == "" || len(task.operation.OperationID) > 128)) {
		w.observe(hook.WorkerExecutionFailed, task.kind)
		return true
	}
	w.mu.Lock()
	_, active := s.active[id]
	w.mu.Unlock()
	if active || s.queued[id] {
		return true
	}
	if len(s.pending[class]) >= hookWorkerPendingPerClass {
		return false
	}
	spaceCount := 0
	for _, queued := range s.pending[class] {
		k, _ := queued.identity()
		if k.WorkspaceID == key.WorkspaceID {
			spaceCount++
		}
	}
	if spaceCount >= hookWorkerPendingPerSpace {
		return false
	}
	if s.pending[class] == nil {
		s.pending[class] = make([]hookWorkerTask, 0, hookWorkerPendingPerClass)
	}
	s.pending[class] = append(s.pending[class], task)
	s.queued[id] = true
	return true
}

// Discovery never consumes an execution turn. Resident candidates keep FIFO
// priority across full-pool rounds; blocked spaces cannot block eligible ones.
// Overflow refill precedes fresh admission; its saved page never loses priority.
func (w *HookWorker) dispatchPending(s *hookWorkerSession, c entity.HookRuntimeConfig) {
	for budget := len(s.queued); budget > 0; budget-- {
		started := false
		for n := 0; n < len(s.pending) && !started; n++ {
			i := (s.nextDispatch + n) % len(s.pending)
			for j, task := range s.pending[i] {
				if s.ctx.Err() != nil {
					return
				}
				if !w.hasCapacity(s, task, c) {
					continue
				}
				current, ok := w.config(s.ctx)
				if !ok {
					return
				}
				c = current
				if !c.WorkerEnabled {
					w.observe(hook.WorkerPaused, "")
					return
				}
				if !w.dispatch(s, task, c) {
					continue
				}
				_, id := task.identity()
				delete(s.queued, id)
				q := s.pending[i]
				copy(q[j:], q[j+1:])
				q[len(q)-1] = hookWorkerTask{}
				s.pending[i] = q[:len(q)-1]
				s.nextDispatch = (i + 1) % len(s.pending)
				started = true
				break
			}
		}
		if !started {
			return
		}
	}
}

func (w *HookWorker) hasCapacity(s *hookWorkerSession, task hookWorkerTask, c entity.HookRuntimeConfig) bool {
	key, id := task.identity()
	w.mu.Lock()
	defer w.mu.Unlock()
	_, active := s.active[id]
	return !active && len(s.active) < int(c.WorkerConcurrency) && s.spaces[key.WorkspaceID] < int(c.WorkspaceConcurrency)
}

func (w *HookWorker) scanPage(s *hookWorkerSession, stream *hookWorkerStream, limit int) []hookWorkerTask {
	ctx, cancel := context.WithTimeout(s.ctx, hookWorkerDependencyTimeout)
	defer cancel()
	if stream.now.IsZero() {
		now, err := w.deps.Clock.Now(ctx)
		if err != nil || ctx.Err() != nil {
			w.observe(hook.WorkerClockFailed, stream.kind)
			return nil
		}
		stream.now = now
	}
	in := entity.HookScanInput{ExecutionScope: w.deps.ExecutionScope, Status: stream.status, Now: stream.now, Limit: limit, Cursor: stream.cursor}
	if in.Validate(stream.kind) != nil {
		stream.now = time.Time{}
		stream.cursor = nil
		w.observe(hook.WorkerClockFailed, stream.kind)
		return nil
	}
	var tasks []hookWorkerTask
	var next *entity.HookScanCursor
	var more bool
	var err error
	switch stream.kind {
	case entity.HookScanDue, entity.HookScanExpired:
		var p entity.HookScanPage[entity.HookOperationCandidate]
		if stream.kind == entity.HookScanDue {
			p, err = w.deps.ScanRepo.ScanDueOperations(ctx, in)
		} else {
			p, err = w.deps.ScanRepo.ScanExpiredOperations(ctx, in)
		}
		if len(p.Candidates) > limit {
			err = ErrHookWorkerConfiguration
		}
		if err == nil {
			for _, candidate := range p.Candidates {
				v := candidate
				tasks = append(tasks, hookWorkerTask{kind: stream.kind, operation: &v})
			}
		}
		next, more = p.NextCursor, p.HasMore
	default:
		var p entity.HookScanPage[entity.HookRunCandidate]
		if stream.kind == entity.HookScanPreparing {
			p, err = w.deps.ScanRepo.ScanPreparingPlans(ctx, in)
		} else {
			p, err = w.deps.ScanRepo.ScanPendingFinalizations(ctx, in)
		}
		if len(p.Candidates) > limit {
			err = ErrHookWorkerConfiguration
		}
		if err == nil {
			for _, v := range p.Candidates {
				tasks = append(tasks, hookWorkerTask{kind: stream.kind, run: v})
			}
		}
		next, more = p.NextCursor, p.HasMore
	}
	if err != nil || ctx.Err() != nil {
		w.observe(hook.WorkerScanFailed, stream.kind)
		return nil
	}
	if more {
		in.Cursor = next
		if next == nil || in.Validate(stream.kind) != nil || (stream.cursor != nil && *next == *stream.cursor) {
			w.observe(hook.WorkerScanFailed, stream.kind)
			return nil
		}
		copy := *next
		stream.cursor = &copy
	} else {
		stream.cursor = nil
		stream.now = time.Time{}
	}
	return tasks
}

func (w *HookWorker) dispatch(s *hookWorkerSession, task hookWorkerTask, c entity.HookRuntimeConfig) bool {
	key, id := task.identity()
	if task.operation != nil {
		if task.operation.OperationID == "" {
			w.observe(hook.WorkerExecutionFailed, task.kind)
			return false
		}
	}
	if key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID <= 0 {
		w.observe(hook.WorkerExecutionFailed, task.kind)
		return false
	}
	w.mu.Lock()
	if s.ctx.Err() != nil {
		w.mu.Unlock()
		return false
	}
	_, duplicate := s.active[id]
	if duplicate || len(s.active) >= int(c.WorkerConcurrency) || s.spaces[key.WorkspaceID] >= int(c.WorkspaceConcurrency) {
		w.mu.Unlock()
		return false
	}
	s.active[id] = key.WorkspaceID
	s.spaces[key.WorkspaceID]++
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		defer func() {
			w.mu.Lock()
			delete(s.active, id)
			s.spaces[key.WorkspaceID]--
			if s.spaces[key.WorkspaceID] == 0 {
				delete(s.spaces, key.WorkspaceID)
			}
			w.mu.Unlock()
		}()
		if s.ctx.Err() != nil {
			return
		}
		if task.operation != nil {
			w.execute(s.ctx, task)
		} else {
			w.coordinate(s.ctx, task)
		}
	}()
	w.mu.Unlock()
	return true
}

func (task hookWorkerTask) identity() (entity.HookRunKey, string) {
	if task.operation != nil {
		return task.operation.Key, "operation/" + task.operation.OperationID
	}
	k := task.run.Key
	return k, "run/" + string(task.kind) + "/" + strconv.FormatInt(k.WorkspaceID, 10) + "/" + strconv.FormatInt(k.ExperimentID, 10) + "/" + strconv.FormatInt(k.RunID, 10)
}

func (w *HookWorker) execute(ctx context.Context, task hookWorkerTask) {
	candidate := task.operation
	idCtx, cancel := context.WithTimeout(ctx, hookWorkerDependencyTimeout)
	id, err := w.deps.IDs.GenID(idCtx)
	cancel()
	if err != nil || id <= 0 {
		w.observe(hook.WorkerIDFailed, task.kind)
		return
	}
	if ctx.Err() != nil {
		return
	}
	result, err := w.deps.Executor.Execute(ctx, hook.AttemptExecutionInput{Key: candidate.Key, ExecutionScope: w.deps.ExecutionScope, OperationID: candidate.OperationID, Phase: candidate.Phase, Owner: w.deps.Owner, AttemptID: id})
	if err != nil {
		w.observe(hook.WorkerExecutionFailed, task.kind)
		return
	}
	if result.RecoveryPending {
		w.observe(hook.WorkerRecoveryPending, task.kind)
	}
	if !result.Changed {
		return
	}
	if result.Run == nil || result.Run.State.Key != candidate.Key {
		w.observe(hook.WorkerExecutionFailed, task.kind)
		return
	}
	effects := hook.WorkerEffects{ExecutionScope: w.deps.ExecutionScope, Key: candidate.Key, OperationID: candidate.OperationID, BeginFinalize: result.Effects.BeginFinalize, ActivateAfter: result.Effects.ActivateAfter, WakeEvaluation: candidate.Phase == entity.HookPhaseBefore && result.Run.State.Gate == entity.HookGateReady}
	if effects.WakeEvaluation || effects.BeginFinalize || effects.ActivateAfter {
		w.deliverEffects(ctx, task.kind, effects)
	}
}

func (w *HookWorker) coordinate(ctx context.Context, task hookWorkerTask) {
	ctx, cancel := context.WithTimeout(ctx, hookWorkerDependencyTimeout)
	defer cancel()
	in := hook.WorkerRunInput{ExecutionScope: w.deps.ExecutionScope, Candidate: task.run}
	var err error
	if task.kind == entity.HookScanPreparing {
		err = w.deps.Coordinator.PreparePlan(ctx, in)
	} else {
		err = w.deps.Coordinator.FinalizeRun(ctx, in)
	}
	if err != nil || ctx.Err() != nil {
		w.observe(hook.WorkerCoordinationFailed, task.kind)
	}
}

func (w *HookWorker) deliverEffects(ctx context.Context, kind entity.HookScanKind, effects hook.WorkerEffects) {
	for attempt := 0; attempt < hookWorkerEffectAttempts; attempt++ {
		if ctx.Err() != nil {
			w.observe(hook.WorkerEffectsAbandoned, kind)
			return
		}
		callCtx, cancel := context.WithTimeout(ctx, hookWorkerDependencyTimeout)
		err := w.deps.Coordinator.ApplyEffects(callCtx, effects)
		callErr := callCtx.Err()
		cancel()
		if err == nil && callErr == nil {
			return
		}
		if attempt == hookWorkerEffectAttempts-1 {
			w.observe(hook.WorkerEffectsExhausted, kind)
			return
		}
		w.observe(hook.WorkerEffectsRetry, kind)
		timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			w.observe(hook.WorkerEffectsAbandoned, kind)
			return
		case <-timer.C:
		}
	}
}
