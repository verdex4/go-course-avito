package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/verdex4/go-course-avito/internal/model"
)

const tracerName = "trip-service/worker/positions"

type Config struct {
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	RequestTimeout time.Duration
}

type task struct {
	tripID   uuid.UUID
	driverID uuid.UUID
	spanLink trace.Link
}

type Pool struct {
	repo    staleTripsRepo
	gateway PushGateway
	cfg     Config
	logger  *slog.Logger
	metrics poolMetrics

	queue   chan task
	inQueue map[uuid.UUID]struct{}
	mu      sync.Mutex

	scanning bool

	newTicker       func(d time.Duration) Ticker
	onBeforeProcess func(tripID uuid.UUID)
	wg              sync.WaitGroup
}

func NewPool(
	repo staleTripsRepo,
	gateway PushGateway,
	cfg Config,
	metrics poolMetrics,
	logger *slog.Logger,
) *Pool {
	if logger == nil {
		logger = slog.Default()
	}
	return &Pool{
		repo:      repo,
		gateway:   gateway,
		cfg:       cfg,
		logger:    logger,
		metrics:   metrics,
		queue:     make(chan task, cfg.QueueSize),
		inQueue:   make(map[uuid.UUID]struct{}),
		newTicker: newRealTicker,
	}
}

func (p *Pool) WithTicker(fn func(d time.Duration) Ticker) *Pool {
	p.newTicker = fn
	return p
}

func (p *Pool) WithBeforeProcess(fn func(tripID uuid.UUID)) *Pool {
	p.onBeforeProcess = fn
	return p
}

func (p *Pool) Start(ctx context.Context) {
	p.logger.Info("positions pool starting", "workers", p.cfg.Workers, "queue_size", p.cfg.QueueSize)

	p.metrics.RegisterQueueSizeGauge(func() int64 {
		return int64(len(p.queue))
	})

	for i := range p.cfg.Workers {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}

	p.runScanner(ctx)
	close(p.queue)
	p.wg.Wait()

	p.logger.Info("positions pool stopped")
}

func (p *Pool) Cancel(tripID uuid.UUID) {
	p.mu.Lock()
	delete(p.inQueue, tripID)
	p.mu.Unlock()
}

func (p *Pool) runScanner(ctx context.Context) {
	ticker := p.newTicker(p.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case tickTime, ok := <-ticker.C():
			if !ok {
				return
			}

			p.mu.Lock()
			if p.scanning {
				p.mu.Unlock()
				p.logger.Debug("scanner tick skipped: previous scan still running")
				continue
			}
			p.scanning = true
			p.mu.Unlock()

			p.scan(ctx, tickTime)

			p.mu.Lock()
			p.scanning = false
			p.mu.Unlock()
		}
	}
}

func (p *Pool) scan(ctx context.Context, _ time.Time) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "scanner.scan")
	defer span.End()

	trips, err := p.repo.GetStaleTrips(ctx, p.cfg.StaleAfter)
	if err != nil {
		p.logger.Error("failed to get stale trips", "error", err)
		return
	}

	for _, t := range trips {
		p.tryEnqueue(ctx, t, span)
	}
}

func (p *Pool) tryEnqueue(ctx context.Context, t *model.Trip, scanSpan trace.Span) {
	tracer := otel.Tracer(tracerName)
	_, enqueueSpan := tracer.Start(ctx, "scanner.enqueue",
		trace.WithAttributes(),
	)
	defer enqueueSpan.End()

	spanLink := trace.Link{SpanContext: scanSpan.SpanContext()}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.inQueue[t.ID]; ok {
		p.metrics.AddStaleCheck(ctx, "duplicate")
		return
	}

	tsk := task{
		tripID:   t.ID,
		driverID: t.DriverID,
		spanLink: spanLink,
	}

	select {
	case p.queue <- tsk:
		p.inQueue[t.ID] = struct{}{}
		p.metrics.AddStaleCheck(ctx, "queued")
	default:
		p.metrics.AddStaleCheck(ctx, "dropped")
		p.logger.Warn("position queue full, task dropped", "trip_id", t.ID.String())
	}
}

func (p *Pool) runWorker(ctx context.Context, id int) {
	defer p.wg.Done()
	p.logger.Debug("worker started", "worker_id", id)

	for tsk := range p.queue {
		p.processTask(ctx, tsk)
	}

	p.logger.Debug("worker stopped", "worker_id", id)
}

func (p *Pool) processTask(ctx context.Context, tsk task) {
	tracer := otel.Tracer(tracerName)
	_, span := tracer.Start(ctx, "worker.processTask",
		trace.WithLinks(tsk.spanLink),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	p.metrics.IncWorkersActive(ctx)
	defer p.metrics.DecWorkersActive(ctx)

	if p.onBeforeProcess != nil {
		p.onBeforeProcess(tsk.tripID)
	}

	p.mu.Lock()
	_, active := p.inQueue[tsk.tripID]
	p.mu.Unlock()

	if !active {
		p.metrics.AddPositionRequest(ctx, "dropped")
		return
	}

	start := time.Now()

	reqCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	err := p.gateway.RequestPosition(reqCtx, tsk.driverID, tsk.tripID)
	duration := time.Since(start).Seconds()

	if err != nil {
		p.metrics.AddPositionRequest(ctx, "error")
		p.metrics.RecordPositionRequestDuration(ctx, duration, "error")
		p.logger.Warn("push request failed", "trip_id", tsk.tripID.String(), "error", err)
	} else {
		p.metrics.AddPositionRequest(ctx, "success")
		p.metrics.RecordPositionRequestDuration(ctx, duration, "success")
	}

	p.mu.Lock()
	delete(p.inQueue, tsk.tripID)
	p.mu.Unlock()
}
