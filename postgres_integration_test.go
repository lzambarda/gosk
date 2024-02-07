//go:build integration

package gosk_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lzambarda/gosk"
)

type dbTask struct {
	ID          int64      `db:"id"`
	CreatedAt   *time.Time `db:"created_at"`
	CompletedAt *time.Time `db:"completed_at"`
	CancelledAt *time.Time `db:"cancelled_at"`
	PingedAt    *time.Time `db:"pinged_at"`
	Content     task       `db:"content"`
}

type task struct {
	Input  string `db:"input"`
	Output string `db:"output"`
}

//nolint:gocyclo // Fine here.
func TestPostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	const connstring = "postgres://gosk_test:gosk_test@localhost:5432/gosk_test"

	cfg, err := pgxpool.ParseConfig(connstring)
	if err != nil {
		t.Fatalf("ParseConfig: %s", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %s", err)
	}
	_, err = pool.Exec(ctx, "DROP SCHEMA IF EXISTS gosk CASCADE;")
	if err != nil {
		t.Fatalf("drop schema: %s", err)
	}

	const safeWindow = time.Second
	p, err := gosk.NewPostgres[task](ctx, connstring, 1, safeWindow)
	if err != nil {
		t.Fatalf("new postgres: %s", err)
	}

	err = p.Init(ctx)
	if err != nil {
		t.Fatalf("init: %s", err)
	}

	_, _, err = p.GetTask(ctx, gosk.Fifo)
	if !errors.Is(err, gosk.ErrNoTask) {
		t.Fatalf("expecting ErrNoTask, got: %s", err)
	}

	task1 := task{
		Input:  "input_1",
		Output: "output_1",
	}
	task1CreatedAt := time.Now()
	err = p.CreateTask(ctx, task1)
	if err != nil {
		t.Fatalf("create task 1: %s", err)
	}

	task2 := task{
		Input:  "input_2",
		Output: "output_2",
	}
	task2CreatedAt := time.Now()
	err = p.CreateTask(ctx, task2)
	if err != nil {
		t.Fatalf("create task 2: %s", err)
	}

	actualTaskID1, actualTask1, err := p.GetTask(ctx, gosk.Fifo)
	if err != nil {
		t.Fatalf("get task 1: %s", err)
	}

	if actualTaskID1 != 1 {
		t.Errorf("task id 1 should be 1, got: %d", actualTaskID1)
	}

	if diff := cmp.Diff(task1, actualTask1); diff != "" {
		t.Errorf("task 1 (-expected,+actual):\n%s", diff)
	}

	time.Sleep(time.Second)

	task1PingedAt := time.Now()
	err = p.PingTask(ctx, 1)
	if err != nil {
		t.Fatalf("ping task 1: %s", err)
	}

	time.Sleep(time.Second)

	task1CancelledAt := time.Now()
	err = p.CancelTask(ctx, 1)
	if err != nil {
		t.Errorf("cancel task 1: %s", err)
	}

	err = p.CompleteTask(ctx, 1)
	if err != nil && !errors.Is(err, gosk.ErrTaskConflict) {
		t.Errorf("should get a conflict on completing a cancelled task, got: %s", err)
	}

	task2GotAt := time.Now()
	actualTaskID2, actualTask2, err := p.GetTask(ctx, gosk.Fifo)
	if err != nil {
		t.Errorf("get task 2: %s", err)
	}

	if actualTaskID2 != 2 {
		t.Errorf("task id 2 should be 1, got: %d", actualTaskID2)
	}

	if diff := cmp.Diff(task2, actualTask2); diff != "" {
		t.Errorf("task 2 (-expected,+actual):\n%s", diff)
	}

	time.Sleep(time.Second)
	task2CompletedAt := time.Now()
	err = p.CompleteTask(ctx, 2)
	if err != nil {
		t.Errorf("complete task 2: %s", err)
	}

	rows, err := pool.Query(ctx, `SELECT * FROM gosk.task ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("query table: %s", err)
	}
	actualTasks, err := pgx.CollectRows[dbTask](rows, pgx.RowToStructByName[dbTask])
	if err != nil {
		t.Fatalf("collect rows: %s", err)
	}
	expectedTasks := []dbTask{
		{
			ID:          1,
			CreatedAt:   &task1CreatedAt,
			PingedAt:    &task1PingedAt,
			CancelledAt: &task1CancelledAt,
			Content:     task{Input: "input_1", Output: "output_1"},
		},
		{
			ID:          2,
			CreatedAt:   &task2CreatedAt,
			PingedAt:    &task2GotAt,
			CompletedAt: &task2CompletedAt,
			Content:     task{Input: "input_2", Output: "output_2"},
		},
	}

	if diff := cmp.Diff(expectedTasks, actualTasks, cmpopts.EquateApproxTime(time.Millisecond*100)); diff != "" {
		t.Errorf("tasks (-expected,+actual):\n%s", diff)
	}
}
