package agent

import (
	"context"
	"sync"

	"github.com/aslakknutsen/kkbase/pkg/agenttypes"
	"go.uber.org/zap"
)

// WorkerPool manages a pool of agent workers
type WorkerPool struct {
	workers     int
	eventQueue  chan agenttypes.ProcessedEvent
	resultQueue chan agenttypes.InvestigationResult
	agents      []*Agent
	logger      *zap.Logger
	wg          sync.WaitGroup
	stopCh      chan struct{}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(numWorkers int, agents []*Agent, logger *zap.Logger) *WorkerPool {
	return &WorkerPool{
		workers:     numWorkers,
		eventQueue:  make(chan agenttypes.ProcessedEvent, numWorkers*10),
		resultQueue: make(chan agenttypes.InvestigationResult, numWorkers*10),
		agents:      agents,
		logger:      logger,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the worker pool
func (p *WorkerPool) Start(ctx context.Context) error {
	p.logger.Info("starting worker pool", zap.Int("workers", p.workers))

	// Start worker goroutines
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	// Start result logger
	go p.resultLogger(ctx)

	return nil
}

// Submit submits an event to the worker pool
func (p *WorkerPool) Submit(event agenttypes.ProcessedEvent) error {
	select {
	case p.eventQueue <- event:
		return nil
	default:
		return ErrQueueFull
	}
}

// Results returns the results channel
func (p *WorkerPool) Results() <-chan agenttypes.InvestigationResult {
	return p.resultQueue
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	close(p.stopCh)
	close(p.eventQueue)
	p.wg.Wait()
	close(p.resultQueue)
}

// worker processes events from the queue
func (p *WorkerPool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	agent := p.agents[id]

	p.logger.Info("worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker stopping (context done)", zap.Int("worker_id", id))
			return
		case <-p.stopCh:
			p.logger.Info("worker stopping", zap.Int("worker_id", id))
			return
		case event, ok := <-p.eventQueue:
			if !ok {
				p.logger.Info("worker stopping (queue closed)", zap.Int("worker_id", id))
				return
			}

			p.logger.Info("worker processing event",
				zap.Int("worker_id", id),
				zap.String("event_id", event.Event.ID),
				zap.String("reason", event.Event.Reason))

			// Investigate the event
			result := agent.Investigate(ctx, event.Event)

			// Send result
			select {
			case p.resultQueue <- result:
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			default:
				p.logger.Warn("result queue full, dropping result",
					zap.String("event_id", event.Event.ID))
			}
		}
	}
}

// resultLogger logs investigation results
func (p *WorkerPool) resultLogger(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case result, ok := <-p.resultQueue:
			if !ok {
				return
			}

			if result.Error != nil {
				p.logger.Error("investigation failed",
					zap.String("event_id", result.Event.ID),
					zap.Error(result.Error),
					zap.Duration("duration", result.Duration))
			} else {
				p.logger.Info("investigation completed",
					zap.String("event_id", result.Event.ID),
					zap.String("session_id", result.SessionID),
					zap.String("status", result.Status),
					zap.Int("recommendations", len(result.Recommendations)),
					zap.Duration("duration", result.Duration))
			}
		}
	}
}

// ErrQueueFull is returned when the event queue is full
var ErrQueueFull = &QueueFullError{}

// QueueFullError represents a full queue error
type QueueFullError struct{}

func (e *QueueFullError) Error() string {
	return "event queue is full"
}
