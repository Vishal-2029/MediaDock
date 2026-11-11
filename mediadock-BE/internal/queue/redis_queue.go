package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mediadock/backend/internal/cache"
	"github.com/redis/go-redis/v9"
)

const (
	defaultQueueKey = "mediadock:jobs:pending"
	deadLetterKey   = "mediadock:jobs:dlq"
)

// JobType identifies the kind of background task.
type JobType string

const (
	JobTypeDownload   JobType = "download"
	JobTypeConversion JobType = "conversion"
)

// JobPayload wraps a queued task with retry metadata.
type JobPayload struct {
	ID         uuid.UUID `json:"id"`
	Type       JobType   `json:"type"`
	Attempt    int       `json:"attempt"`
	MaxRetries int       `json:"max_retries"`
	Token      string    `json:"token,omitempty"` // Ephemeral per-request OAuth token; never logged
	EnqueuedAt time.Time `json:"enqueued_at"`
}

// JobHandler processes a single dequeued job.
type JobHandler func(ctx context.Context, payload JobPayload) error

// WorkerQueue manages Redis-backed job enqueueing and concurrent worker execution.
type WorkerQueue struct {
	redis       *cache.RedisClient
	log         *slog.Logger
	concurrency int
	jobTimeout  time.Duration
	maxRetries  int

	mu       sync.RWMutex
	handlers map[JobType]JobHandler
	wg       sync.WaitGroup
}

// NewWorkerQueue constructs a Redis-backed background job worker pool.
func NewWorkerQueue(redisClient *cache.RedisClient, log *slog.Logger, concurrency int, jobTimeout time.Duration, maxRetries int) *WorkerQueue {
	if concurrency <= 0 {
		concurrency = 2
	}
	if jobTimeout <= 0 {
		jobTimeout = 5 * time.Minute
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &WorkerQueue{
		redis:       redisClient,
		log:         log,
		concurrency: concurrency,
		jobTimeout:  jobTimeout,
		maxRetries:  maxRetries,
		handlers:    make(map[JobType]JobHandler),
	}
}

// RegisterHandler binds a handler function to a JobType.
func (q *WorkerQueue) RegisterHandler(jobType JobType, handler JobHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[jobType] = handler
}

// Enqueue pushes a job onto the Redis pending queue.
func (q *WorkerQueue) Enqueue(ctx context.Context, jobType JobType, jobID uuid.UUID, ephemeralToken string) error {
	payload := JobPayload{
		ID:         jobID,
		Type:       jobType,
		Attempt:    1,
		MaxRetries: q.maxRetries,
		Token:      ephemeralToken,
		EnqueuedAt: time.Now().UTC(),
	}
	return q.push(ctx, defaultQueueKey, payload)
}

func (q *WorkerQueue) push(ctx context.Context, listKey string, payload JobPayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	return q.redis.Client.RPush(ctx, listKey, raw).Err()
}

// Start launches concurrency worker goroutines and blocks until ctx is canceled,
// waiting for in-flight jobs to drain cleanly.
func (q *WorkerQueue) Start(ctx context.Context) {
	for i := 0; i < q.concurrency; i++ {
		workerID := i + 1
		q.wg.Add(1)
		go func(id int) {
			defer q.wg.Done()
			q.workerLoop(ctx, id)
		}(workerID)
	}
}

// Wait blocks until all worker goroutines exit.
func (q *WorkerQueue) Wait() {
	q.wg.Wait()
}

func (q *WorkerQueue) workerLoop(ctx context.Context, workerID int) {
	for {
		if ctx.Err() != nil {
			return
		}

		res, err := q.redis.Client.BLPop(ctx, 2*time.Second, defaultQueueKey).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			if ctx.Err() == nil && q.log != nil {
				q.log.Warn("queue pop error", "worker", workerID, "error", err.Error())
				time.Sleep(500 * time.Millisecond)
			}
			continue
		}

		if len(res) < 2 {
			continue
		}

		var payload JobPayload
		if err := json.Unmarshal([]byte(res[1]), &payload); err != nil {
			if q.log != nil {
				q.log.Error("corrupt job payload discarded", "worker", workerID, "error", err.Error())
			}
			continue
		}

		q.ProcessOne(ctx, payload)
	}
}

// ProcessOne executes a single job payload with timeout and retry handling (exported for tests).
func (q *WorkerQueue) ProcessOne(ctx context.Context, payload JobPayload) {
	q.mu.RLock()
	handler, ok := q.handlers[payload.Type]
	q.mu.RUnlock()

	if !ok {
		if q.log != nil {
			q.log.Error("no handler registered for job type", "type", payload.Type, "job_id", payload.ID)
		}
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, q.jobTimeout)
	defer cancel()

	err := handler(jobCtx, payload)
	if err == nil {
		return
	}

	if q.log != nil {
		q.log.Warn("job execution failed",
			"job_id", payload.ID,
			"type", payload.Type,
			"attempt", payload.Attempt,
			"max_retries", payload.MaxRetries,
			"error", err.Error(),
		)
	}

	if payload.Attempt < payload.MaxRetries && ctx.Err() == nil {
		payload.Attempt++
		backoff := time.Duration(payload.Attempt) * 2 * time.Second
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		_ = q.push(ctx, defaultQueueKey, payload)
		return
	}

	// Exhausted retries: move metadata (without token) to DLQ for diagnostics.
	payload.Token = ""
	_ = q.push(context.Background(), deadLetterKey, payload)
}

