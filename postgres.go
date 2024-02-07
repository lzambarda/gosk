package gosk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is a driver connecting with a postgreSQL database. Obtain a usable
// instance with NewPostgres.
type Postgres[T any] struct {
	pool       *pgxpool.Pool
	safeWindow time.Duration
}

// NewPostgres returns a usable instance of Postgres. T is the type of task.
//
// SafeWindow is the minimum amount of time which must have passed after a task
// was last pinged in order for this Driver to return it.
//
// Type T must support JSON marshalling so that this can be stored in the DB
// (see Init task for more details about the schema).
func NewPostgres[T any](ctx context.Context, connstring string, maxConns int32, safeWindow time.Duration) (*Postgres[T], error) {
	p := &Postgres[T]{
		safeWindow: safeWindow,
	}

	cfg, err := pgxpool.ParseConfig(connstring)
	if err != nil {
		return nil, fmt.Errorf("parse connstring: %w", err)
	}
	cfg.MaxConns = maxConns
	p.pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect with config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	err = p.pool.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return p, nil
}

// Init initialises the underlying technology used to store tasks.
func (p *Postgres[T]) Init(ctx context.Context) error {
	const schemaQuery = `CREATE SCHEMA IF NOT EXISTS gosk;`
	_, err := p.pool.Exec(ctx, schemaQuery)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	const tableQuery = `CREATE TABLE IF NOT EXISTS gosk.task (
		id BIGSERIAL PRIMARY KEY,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMPTZ,
		cancelled_at TIMESTAMPTZ,
		pinged_at TIMESTAMPTZ,
		content JSONB NOT NULL
	);`
	_, err = p.pool.Exec(ctx, tableQuery)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	return nil
}

// CreateTask creates a new task with the given content as payload.
func (p *Postgres[T]) CreateTask(ctx context.Context, content T) error {
	const query = `INSERT INTO gosk.task (content) VALUES ($1::jsonb);`
	_, err := p.pool.Exec(ctx, query, content)
	if err != nil {
		return fmt.Errorf("insert into table: %w", err)
	}

	return nil
}

func getQueryForRule(rule PriorityRule) (string, error) {
	switch rule {
	case Fifo:
		return `ORDER BY created_at ASC`, nil
	default:
		return "", fmt.Errorf("%w: %v", ErrUnsupportedPriorityRule, rule)
	}
}

// GetTask returns the next available task, using the given PriorityRule. If
// no task is found this returnes ErrNoTask.
//
// All PriorityRule values should be implemented.
func (p *Postgres[T]) GetTask(ctx context.Context, rule PriorityRule) (taskID int64, taskContent T, err error) {
	const getQuery = `UPDATE gosk.task
	SET pinged_at = NOW()
	FROM (
		SELECT id FROM gosk.task
		WHERE completed_at IS NULL
			AND cancelled_at IS NULL
			AND (pinged_at IS NULL OR NOW() - pinged_at > $1::interval)
		%s
		LIMIT 1
	) t
	WHERE t.id = task.id
	RETURNING task.id,task.content`

	orderStatement, err := getQueryForRule(rule)
	if err != nil {
		return taskID, taskContent, fmt.Errorf("get query for rule: %w", err)
	}

	err = p.pool.QueryRow(ctx, fmt.Sprintf(getQuery, orderStatement), p.safeWindow).Scan(&taskID, &taskContent)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return taskID, taskContent, ErrNoTask
		}
		return taskID, taskContent, fmt.Errorf("query from table: %w", err)
	}

	return taskID, taskContent, nil
}

// PingTask updates the status of a task to mark it as still being hold by a
// worker. This is important to prevent other workers from picking up the
// same task.
func (p *Postgres[T]) PingTask(ctx context.Context, taskID int64) error {
	const query = `UPDATE gosk.task SET pinged_at = NOW() WHERE id=$1::bigint`
	_, err := p.pool.Exec(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

// CompleteTask marks a non-finalised task as completed. It returns
// ErrTaskConflict if the task was already finalised.
func (p *Postgres[T]) CompleteTask(ctx context.Context, taskID int64) error {
	const query = `UPDATE gosk.task
	SET completed_at = NOW()
	WHERE id=$1::bigint AND completed_at IS NULL AND cancelled_at IS NULL`

	tag, err := p.pool.Exec(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("complete task %d: %w", taskID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cancel task %d: %w", taskID, ErrTaskConflict)
	}

	return nil
}

// CancelTask a non-finalised task as cancelled. It returns
// ErrTaskConflict if the task was already finalised.
func (p *Postgres[T]) CancelTask(ctx context.Context, taskID int64) error {
	const query = `UPDATE gosk.task
	SET cancelled_at = NOW()
	WHERE id=$1::bigint AND completed_at IS NULL AND cancelled_at IS NULL`

	tag, err := p.pool.Exec(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("cancel task %d: %w", taskID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cancel task %d: %w", taskID, ErrTaskConflict)
	}

	return nil
}
