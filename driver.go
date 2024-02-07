package gosk

import (
	"context"
)

// TaskFunc is called by a worker whenever a task is successfully found.
//
// If an error is returned, the worker will give up the task which is still
// valid,
//
// If the returned error contains gosk.ErrCancelTask then the worker will mark
// the task as cancelled. It can no longer be picked up by other workers.
type TaskFunc[T any] func(task T) (err error)

// Driver is the main interface to a data storage solution used by a Worker.
//
//go:generate mockery --name Driver --structname Driver --filename driver_mock.go
type Driver[T, S any] interface {
	// Init initialises the underlying technology used to store tasks.
	Init(ctx context.Context) error
	// CreateTask creates a new task with the given content as payload.
	CreateTask(ctx context.Context, content T) error
	// GetTask returns the next available task, using the given PriorityRule. If
	// no task is found this returnes ErrNoTask.
	//
	// All PriorityRule values should be implemented.
	GetTask(ctx context.Context, rule PriorityRule) (taskID S, taskContent T, err error)
	// PingTask updates the status of a task to mark it as still being hold by a
	// worker. This is important to prevent other workers from picking up the
	// same task.
	PingTask(ctx context.Context, taskID S) error
	// CompleteTask marks a non-finalised task as completed. It returns
	// ErrTaskConflict if the task was already finalised.
	CompleteTask(ctx context.Context, taskID S) error
	// CancelTask a non-finalised task as cancelled. It returns
	// ErrTaskConflict if the task was already finalised.
	CancelTask(ctx context.Context, taskID S) error
}

// PriorityRule represents the rule used by a Driver to get a task.
type PriorityRule int

const (
	// Invalid is not a valid PriorityRule and cannot be used.
	Invalid PriorityRule = iota
	// Fifo should try to return the oldest, not finalised task.
	Fifo
)
