// Package gosk implements all logic of this library.
package gosk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// Worker is the main struct used by this library. It represents a worker which
// can pick up tasks, run callbacks on them and then finalise them using a
// Driver.
//
// The generic type T is the type of the task.
//
// The generic type S is the type of the identifier of a task as per the Driver.
//
// Obtain a usable instance with NewWorker.
type Worker[T, S any] struct {
	driver   Driver[T, S]
	pollTime time.Duration
	pingTime time.Duration
}

// NewWorker returns a usable instance of Worker which will poll the Driver for
// new tasks at least every pollTime and will then ping a picked-up task every
// pingTime.
func NewWorker[T, S any](driver Driver[T, S], pollTime, pingTime time.Duration) *Worker[T, S] {
	return &Worker[T, S]{
		driver:   driver,
		pollTime: pollTime,
		pingTime: pingTime,
	}
}

// GetTask starts polling the Driver for available tasks using the given
// PriorityRule. Once a valid task is returned by the Driver, a callback
// function is called.
//
// This can be stopped by cancelling the provided context.
func (w *Worker[T, S]) GetTask(ctx context.Context, rule PriorityRule, callback TaskFunc[T]) error {
	pollTicker := time.NewTicker(w.pollTime)
	defer pollTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err() //nolint:wrapcheck // no meaningful info to add.
		case <-pollTicker.C:
		}

		taskID, task, err := w.driver.GetTask(ctx, rule)
		if errors.Is(err, ErrNoTask) {
			time.Sleep(w.pollTime)
			continue
		}
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}

		g, subCtx := errgroup.WithContext(ctx)

		g.Go(func() error {
			ticker := time.NewTicker(w.pingTime)
			for {
				select {
				case <-subCtx.Done():
					ticker.Stop()
					return nil
				case <-ticker.C:
					err = w.driver.PingTask(subCtx, taskID)
					return fmt.Errorf("ping task: %w", err)
				}
			}
		})

		g.Go(func() error {
			err = callback(task)

			if errors.Is(err, ErrCancelTask) {
				cancelErr := w.driver.CancelTask(subCtx, taskID)
				if cancelErr != nil {
					return fmt.Errorf("cancel task: %w", errors.Join(err, cancelErr))
				}
				return nil
			}

			if err != nil {
				return fmt.Errorf("callback: %w", err)
			}

			err = w.driver.CompleteTask(subCtx, taskID)
			if err != nil {
				return fmt.Errorf("complete task: %w", err)
			}

			return nil
		})

		if err = g.Wait(); err != nil {
			return fmt.Errorf("wait: %w", err)
		}
	}
}
