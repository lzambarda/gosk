package gosk

import "errors"

var (
	// ErrNoTask is returned whenever a driver cannot find any tasks.
	ErrNoTask = errors.New("no task found")
	// ErrCancelTask is treated by a worker as a request to cancel the task
	// itself, not just requeue.
	ErrCancelTask = errors.New("cancel task")
	// ErrTaskConflict is returned whenever a task which was already finalised
	// (cancelled, completed) is attempted to be finalised again.
	ErrTaskConflict = errors.New("attempt at finalising already finalised task")
	// ErrUnsupportedPriorityRule is returned whenever a Driver is called with
	// an unimplemented PriorityRule value.
	ErrUnsupportedPriorityRule = errors.New("unsupported priority rule")
)
