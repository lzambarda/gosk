package gosk_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lzambarda/gosk"
	"github.com/lzambarda/gosk/mocks"
	"github.com/stretchr/testify/mock"
)

var anError = errors.New("an error") //nolint:errname,stylecheck // Fine here.

func TestWorker(t *testing.T) {
	t.Run("GetTask", testWorkerGetTask)
}

func testWorkerGetTask(t *testing.T) {
	t.Run("ErrorGetTask", testWorkerGetTaskErrorGetTask)
	t.Run("ErrorPingTask", testWorkerGetTaskErrorPingTask)
	t.Run("ErrorTaskCancel", testWorkerGetTaskErrorTaskCancel)
	t.Run("ErrorTaskGeneric", testWorkerGetTaskErrorTaskGeneric)
	t.Run("Success", testWorkerGetTaskSuccess)
}

func testWorkerGetTaskErrorGetTask(t *testing.T) {
	driver := mocks.NewDriver[string, int32](t)

	worker := gosk.NewWorker[string, int32](driver, time.Second, time.Second)

	ctx := context.Background()

	driver.On("GetTask", ctx, gosk.Fifo).Return(int32(0), "", anError).Once()
	err := worker.GetTask(ctx, gosk.Fifo, func(task string) (err error) {
		t.Errorf("this should never be called if driver.GetTask fails")
		return nil
	})
	if err == nil {
		t.Error("this should error out, nil instead")
	}
}

func testWorkerGetTaskErrorPingTask(t *testing.T) {
	driver := mocks.NewDriver[string, int32](t)

	worker := gosk.NewWorker[string, int32](driver, time.Second, time.Second)

	driver.On("GetTask", mock.Anything, gosk.Fifo).Return(int32(1), "task_1", nil).Once()
	driver.On("PingTask", mock.Anything, int32(1)).Return(anError).Once()
	driver.On("CompleteTask", mock.Anything, int32(1)).Return(nil).Once()
	err := worker.GetTask(context.Background(), gosk.Fifo, func(task string) (err error) {
		// wait enough to get a ping
		time.Sleep(time.Second * 2)
		return nil
	})
	if err == nil {
		t.Error("this should error out, nil instead")
	}
}

func testWorkerGetTaskErrorTaskCancel(t *testing.T) {
	driver := mocks.NewDriver[string, int32](t)

	worker := gosk.NewWorker[string, int32](driver, time.Second, time.Second)

	driver.On("GetTask", mock.Anything, gosk.Fifo).Return(int32(1), "task_1", nil).Once()
	driver.On("PingTask", mock.Anything, int32(1)).Return(anError).Once()
	driver.On("CancelTask", mock.Anything, int32(1)).Return(nil).Once()
	err := worker.GetTask(context.Background(), gosk.Fifo, func(task string) (err error) {
		return gosk.ErrCancelTask
	})
	if err == nil {
		t.Error("this should error out, nil instead")
	}
}

func testWorkerGetTaskErrorTaskGeneric(t *testing.T) {
	driver := mocks.NewDriver[string, int32](t)

	worker := gosk.NewWorker[string, int32](driver, time.Second, time.Second)

	driver.On("GetTask", mock.Anything, gosk.Fifo).Return(int32(1), "task_1", nil).Once()
	// driver.On("PingTask", mock.Anything, int32(1)).Return(anError).Once()
	err := worker.GetTask(context.Background(), gosk.Fifo, func(task string) (err error) {
		return anError
	})
	if err == nil {
		t.Error("this should error out, nil instead")
	}
}

func testWorkerGetTaskSuccess(t *testing.T) {
	driver := mocks.NewDriver[string, int32](t)

	worker := gosk.NewWorker[string, int32](driver, time.Second, time.Second)

	driver.On("GetTask", mock.Anything, gosk.Fifo).Return(int32(1), "task_1", nil).Once()
	driver.On("PingTask", mock.Anything, int32(1)).Return(nil).Once()
	driver.On("CompleteTask", mock.Anything, int32(1)).Return(nil).Once()
	err := worker.GetTask(context.Background(), gosk.Fifo, func(task string) (err error) {
		return nil
	})
	if err == nil {
		t.Error("this should error out, nil instead")
	}
}
