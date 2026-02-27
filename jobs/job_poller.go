package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// JobDispatch represents a row from the job_dispatch table
type JobDispatch struct {
	ID        string          `json:"id"`
	JobType   string          `json:"job_type"`
	TargetIPO sql.NullString  `json:"target_ipo_id"`
	Status    string          `json:"status"`
	Priority  int             `json:"priority"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// JobExecutor is a function that executes a specific job type
type JobExecutor func(ctx context.Context, job JobDispatch) error

// JobPoller polls the job_dispatch table and routes jobs to the correct Go handler
type JobPoller struct {
	db        *sql.DB
	executors map[string]JobExecutor
	interval  time.Duration
	stopChan  chan struct{}
	stopOnce  sync.Once
	mu        sync.RWMutex
	running   bool
}

// NewJobPoller creates a new poller that checks for pending jobs
func NewJobPoller(db *sql.DB, pollInterval time.Duration) *JobPoller {
	return &JobPoller{
		db:        db,
		executors: make(map[string]JobExecutor),
		interval:  pollInterval,
		stopChan:  make(chan struct{}),
	}
}

// RegisterExecutor maps a job_type string to a Go function
func (p *JobPoller) RegisterExecutor(jobType string, executor JobExecutor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.executors[jobType] = executor
	logrus.WithField("job_type", jobType).Info("Registered job executor")
}

// Start begins polling for pending jobs
func (p *JobPoller) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	logrus.WithField("interval", p.interval.String()).Info("Starting Supabase job poller")

	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		// Poll immediately on start
		p.pollAndExecute()

		for {
			select {
			case <-ticker.C:
				p.pollAndExecute()
			case <-p.stopChan:
				logrus.Info("Job poller stopped")
				return
			}
		}
	}()
}

// Stop gracefully stops the poller
func (p *JobPoller) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		logrus.Info("Job poller shutdown complete")
	})
}

// pollAndExecute fetches pending jobs and executes them
func (p *JobPoller) pollAndExecute() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jobs, err := p.claimPendingJobs(ctx, 5) // Process up to 5 jobs per poll
	if err != nil {
		logrus.WithError(err).Error("Failed to claim pending jobs")
		return
	}

	if len(jobs) == 0 {
		return
	}

	logrus.WithField("count", len(jobs)).Debug("Processing dispatched jobs")

	for _, job := range jobs {
		p.executeJob(job)
	}
}

// claimPendingJobs atomically claims pending jobs (prevents double-processing)
func (p *JobPoller) claimPendingJobs(ctx context.Context, limit int) ([]JobDispatch, error) {
	query := `
		UPDATE job_dispatch
		SET status = 'running', picked_up_at = NOW()
		WHERE id IN (
			SELECT id FROM job_dispatch
			WHERE status = 'pending'
			ORDER BY priority DESC, created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, job_type, target_ipo_id, status, priority, payload, created_at
	`

	rows, err := p.db.QueryContext(ctx, query, limit)
	if err != nil {
		// Retry once for transient prepared statement errors
		logrus.WithError(err).Warn("First claim attempt failed, retrying...")
		rows, err = p.db.QueryContext(ctx, query, limit)
		if err != nil {
			return nil, fmt.Errorf("claim pending jobs: %w", err)
		}
	}
	defer rows.Close()

	var jobs []JobDispatch
	for rows.Next() {
		var job JobDispatch
		if err := rows.Scan(&job.ID, &job.JobType, &job.TargetIPO, &job.Status, &job.Priority, &job.Payload, &job.CreatedAt); err != nil {
			logrus.WithError(err).Error("Failed to scan job row")
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// executeJob routes a job to its registered executor
func (p *JobPoller) executeJob(job JobDispatch) {
	p.mu.RLock()
	executor, exists := p.executors[job.JobType]
	p.mu.RUnlock()

	if !exists {
		logrus.WithField("job_type", job.JobType).Warn("No executor registered for job type")
		p.markJobFailed(job.ID, fmt.Sprintf("no executor registered for job type: %s", job.JobType))
		return
	}

	startTime := time.Now()
	logrus.WithFields(logrus.Fields{
		"job_id":   job.ID,
		"job_type": job.JobType,
		"priority": job.Priority,
	}).Info("Executing dispatched job")

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	if err := executor(ctx, job); err != nil {
		duration := time.Since(startTime)
		logrus.WithFields(logrus.Fields{
			"job_id":   job.ID,
			"job_type": job.JobType,
			"duration": duration.String(),
			"error":    err.Error(),
		}).Error("Job execution failed")
		p.markJobFailed(job.ID, err.Error())
		return
	}

	duration := time.Since(startTime)
	logrus.WithFields(logrus.Fields{
		"job_id":   job.ID,
		"job_type": job.JobType,
		"duration": duration.String(),
	}).Info("Job executed successfully")
	p.markJobCompleted(job.ID)
}

func (p *JobPoller) markJobCompleted(jobID string) {
	_, err := p.db.Exec(
		`UPDATE job_dispatch SET status = 'completed', completed_at = NOW() WHERE id = $1`,
		jobID,
	)
	if err != nil {
		logrus.WithError(err).WithField("job_id", jobID).Error("Failed to mark job completed")
	}
}

func (p *JobPoller) markJobFailed(jobID string, errMsg string) {
	_, err := p.db.Exec(
		`UPDATE job_dispatch SET status = 'failed', completed_at = NOW(), error_message = $2 WHERE id = $1`,
		jobID, errMsg,
	)
	if err != nil {
		logrus.WithError(err).WithField("job_id", jobID).Error("Failed to mark job failed")
	}
}
