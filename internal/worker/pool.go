package worker

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
)

// Pool manages a fixed number of worker goroutines that process delivery jobs.
type Pool struct {
	numWorkers int
	jobs       chan engine.DeliveryJob
	deliverer  *Deliverer
	logger     *slog.Logger
	wg         sync.WaitGroup
}

// NewPool creates a worker pool with the given number of workers.
func NewPool(numWorkers int, deliverer *Deliverer, logger *slog.Logger) *Pool {
	return &Pool{
		numWorkers: numWorkers,
		jobs:       make(chan engine.DeliveryJob, numWorkers*2),
		deliverer:  deliverer,
		logger:     logger,
	}
}

// Start launches all worker goroutines. They read from the jobs channel
// until it is closed or the context is cancelled.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	p.logger.Info("worker pool started", "num_workers", p.numWorkers)
}

// Submit sends a job to the worker pool via the jobs channel.
func (p *Pool) Submit(job engine.DeliveryJob) {
	p.jobs <- job
}

// Jobs returns the jobs channel for the dispatcher to send work into.
func (p *Pool) Jobs() chan<- engine.DeliveryJob {
	return p.jobs
}

// Stop closes the jobs channel and waits for all workers to finish.
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	p.logger.Info("worker pool stopped")
}

// worker is a single goroutine that processes jobs from the channel.
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	for job := range p.jobs {
		select {
		case <-ctx.Done():
			return
		default:
			p.deliverer.Deliver(ctx, job)
		}
	}
}
