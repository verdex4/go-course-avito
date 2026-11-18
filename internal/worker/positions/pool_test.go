package positions_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/verdex4/go-course-avito/internal/model"
	"github.com/verdex4/go-course-avito/internal/worker/positions"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type fakeTicker struct {
	ch chan time.Time
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{ch: make(chan time.Time, 1)}
}

func (f *fakeTicker) C() <-chan time.Time { return f.ch }
func (f *fakeTicker) Stop()               {}
func (f *fakeTicker) Tick()               { f.ch <- time.Now() }

type fakeRepo struct {
	mu    sync.Mutex
	trips []*model.Trip
}

func (r *fakeRepo) GetStaleTrips(_ context.Context, _ time.Duration) ([]*model.Trip, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.trips, nil
}

func (r *fakeRepo) setTrips(trips []*model.Trip) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trips = trips
}

type fakeGateway struct {
	mu     sync.Mutex
	calls  []uuid.UUID
	err    error
	onCall func()
}

func (g *fakeGateway) RequestPosition(_ context.Context, _, tripID uuid.UUID) error {
	if g.onCall != nil {
		g.onCall()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, tripID)
	return g.err
}

func (g *fakeGateway) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}

type fakeMetrics struct {
	queued    atomic.Int64
	duplicate atomic.Int64
	dropped   atomic.Int64
}

func (m *fakeMetrics) AddStaleCheck(_ context.Context, result string) {
	switch result {
	case "queued":
		m.queued.Add(1)
	case "duplicate":
		m.duplicate.Add(1)
	case "dropped":
		m.dropped.Add(1)
	}
}

func (m *fakeMetrics) AddPositionRequest(_ context.Context, _ string)                       {}
func (m *fakeMetrics) RecordPositionRequestDuration(_ context.Context, _ float64, _ string) {}
func (m *fakeMetrics) IncWorkersActive(_ context.Context)                                   {}
func (m *fakeMetrics) DecWorkersActive(_ context.Context)                                   {}
func (m *fakeMetrics) RegisterQueueSizeGauge(_ func() int64)                                {}

func newTestPool(repo *fakeRepo, gw *fakeGateway, m *fakeMetrics, workers, queueSize int) (*positions.Pool, *fakeTicker) {
	ft := newFakeTicker()
	cfg := positions.Config{
		CheckInterval:  time.Hour,
		StaleAfter:     time.Minute,
		Workers:        workers,
		QueueSize:      queueSize,
		RequestTimeout: time.Second,
	}
	pool := positions.NewPool(repo, gw, cfg, m, nil)
	pool.WithTicker(func(_ time.Duration) positions.Ticker { return ft })
	return pool, ft
}

func startPool(pool *positions.Pool) (context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		pool.Start(ctx)
	}()
	return cancel, done
}

// --- ТЕСТЫ ---

func TestPool_Deduplication(t *testing.T) {
	tripID := uuid.New()
	trip := &model.Trip{ID: tripID, DriverID: uuid.New()}

	repo := &fakeRepo{}
	repo.setTrips([]*model.Trip{trip})

	workerStarted := make(chan struct{})
	workerProceed := make(chan struct{})
	gw := &fakeGateway{onCall: func() {
		close(workerStarted)
		<-workerProceed
	}}
	metrics := &fakeMetrics{}

	pool, ft := newTestPool(repo, gw, metrics, 1, 10)
	cancel, done := startPool(pool)

	ft.Tick()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int64(1), metrics.queued.Load())

	<-workerStarted

	ft.Tick()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int64(1), metrics.duplicate.Load())

	close(workerProceed)
	cancel()
	<-done
}

func TestPool_CancelPreventsDispatch(t *testing.T) {
	t.Parallel()

	blocker := &model.Trip{ID: uuid.New(), DriverID: uuid.New()}
	target := &model.Trip{ID: uuid.New(), DriverID: uuid.New()}
	sentinel := &model.Trip{ID: uuid.New(), DriverID: uuid.New()}

	repo := &fakeRepo{}
	repo.setTrips([]*model.Trip{blocker})

	blockerStarted := make(chan struct{})
	proceed := make(chan struct{})

	var callSeq atomic.Int64
	gw := &fakeGateway{}
	gw.onCall = func() {
		if callSeq.Add(1) == 1 {
			close(blockerStarted)
			select {
			case <-proceed:
			case <-time.After(2 * time.Second):
			}
		}
	}

	metrics := &fakeMetrics{}
	pool, ft := newTestPool(repo, gw, metrics, 1, 10)
	cancel, done := startPool(pool)
	defer func() { cancel(); <-done }()

	ft.Tick()
	select {
	case <-blockerStarted:
	case <-time.After(time.Second):
		t.Fatal("воркер не взял блокирующую задачу")
	}

	repo.setTrips([]*model.Trip{target})
	ft.Tick()
	require.Eventually(t, func() bool { return metrics.queued.Load() == 2 },
		time.Second, time.Millisecond)

	pool.Cancel(target.ID)

	repo.setTrips([]*model.Trip{sentinel})
	ft.Tick()
	require.Eventually(t, func() bool { return metrics.queued.Load() == 3 },
		time.Second, time.Millisecond)

	close(proceed)
	require.Eventually(t, func() bool {
		gw.mu.Lock()
		defer gw.mu.Unlock()
		return slices.Contains(gw.calls, sentinel.ID)
	}, time.Second, time.Millisecond)

	cancel()
	<-done

	gw.mu.Lock()
	defer gw.mu.Unlock()
	assert.Equal(t, []uuid.UUID{blocker.ID, sentinel.ID}, gw.calls,
		"cancelled trip must not be dispatched")
}

func TestPool_QueueFull_Dropped(t *testing.T) {
	const numTrips = 5
	const queueSize = 2

	blocker := &model.Trip{ID: uuid.New(), DriverID: uuid.New()}
	trips := make([]*model.Trip, numTrips)
	for i := range trips {
		trips[i] = &model.Trip{ID: uuid.New(), DriverID: uuid.New()}
	}

	repo := &fakeRepo{}
	repo.setTrips([]*model.Trip{blocker})

	workerStarted := make(chan struct{})
	proceed := make(chan struct{})
	var callSeq atomic.Int64
	gw := &fakeGateway{}
	gw.onCall = func() {
		if callSeq.Add(1) == 1 {
			close(workerStarted)
			select {
			case <-proceed:
			case <-time.After(2 * time.Second):
			}
		}
	}

	metrics := &fakeMetrics{}
	pool, ft := newTestPool(repo, gw, metrics, 1, queueSize)
	cancel, done := startPool(pool)
	defer func() { cancel(); <-done }()

	ft.Tick()
	select {
	case <-workerStarted:
	case <-time.After(time.Second):
		t.Fatal("воркер не взял блокирующую задачу")
	}

	repo.setTrips(trips)
	ft.Tick()

	require.Eventually(t, func() bool {
		return metrics.queued.Load()+metrics.dropped.Load() == 1+numTrips
	}, time.Second, time.Millisecond)

	assert.Equal(t, int64(1+queueSize), metrics.queued.Load(), "blocker + queueSize задач в очереди")
	assert.Equal(t, int64(numTrips-queueSize), metrics.dropped.Load())

	close(proceed)
	cancel()
	<-done
}

func TestPool_NWorkers(t *testing.T) {
	const numWorkers = 3
	const numTrips = 6

	trips := make([]*model.Trip, numTrips)
	for i := range trips {
		trips[i] = &model.Trip{ID: uuid.New(), DriverID: uuid.New()}
	}

	repo := &fakeRepo{}
	repo.setTrips(trips)

	gw := &fakeGateway{}
	metrics := &fakeMetrics{}

	pool, ft := newTestPool(repo, gw, metrics, numWorkers, numTrips)
	cancel, done := startPool(pool)

	ft.Tick()

	require.Eventually(t, func() bool {
		return gw.callCount() == numTrips
	}, time.Second, time.Millisecond)

	cancel()
	<-done
}

func TestPool_Shutdown_NoGoroutines(t *testing.T) {
	repo := &fakeRepo{}
	repo.setTrips(nil)

	gw := &fakeGateway{}
	metrics := &fakeMetrics{}

	pool, _ := newTestPool(repo, gw, metrics, 2, 10)
	cancel, done := startPool(pool)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pool did not stop in time")
	}
}

func TestPool_GatewayError_WorkerContinues(t *testing.T) {
	trip1 := &model.Trip{ID: uuid.New(), DriverID: uuid.New()}
	trip2 := &model.Trip{ID: uuid.New(), DriverID: uuid.New()}

	repo := &fakeRepo{}
	metrics := &fakeMetrics{}
	gw := &fakeGateway{err: errors.New("push failed")}

	pool, ft := newTestPool(repo, gw, metrics, 1, 10)
	cancel, done := startPool(pool)
	defer func() { cancel(); <-done }()

	repo.setTrips([]*model.Trip{trip1})
	ft.Tick()
	require.Eventually(t, func() bool { return gw.callCount() == 1 }, time.Second, time.Millisecond)

	gw.mu.Lock()
	gw.err = nil
	gw.mu.Unlock()

	repo.setTrips([]*model.Trip{trip2})
	ft.Tick()
	require.Eventually(t, func() bool { return gw.callCount() == 2 }, time.Second, time.Millisecond)

	cancel()
	<-done
}
