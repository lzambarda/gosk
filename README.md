# gosk

Simple library to handle multiple workers working on tasks.

Each [Worker](./worker.go) uses a [Driver](./driver.go) interface to access a data storage.
This interface handles a set of basic operations which are called by each Worker
 to create tasks, pick them up and finalise them (cancellation / completion).

A [Driver for PostgreSQL](./postgres.go) has already been written to provide an
example for how to implement a `Driver`.

## Why?

In order to handle tasks, I have before now used technologies such as RabbitMQ
or AWS SQS. I found out that they did not always matched the requirements I had
an also came with a certain integration/maintenance cost.

This library is an attempt at creating a simple, yet flexible, system to manage
parallel workers.

## Usage and task lifecycle

### Getting started

```sh
make help # is all you need 🎵
```

### Go example

```go

type myTask struct {
 ID        int64  `json:"id"`
 Requester string `json:"requester"`
 Data      []byte `json:"data"`
}

...

pgdriver, err := gosk.NewPostgres[myTask](ctx, "postgres://user:pass@host:port/db", 1, time.Minute)
if err != nil {
    log.Fatal("new postgres driver", err)
}

err = pgdriver.CreateTask(ctx, myTask{
    ID:        1,
    Requester: "foo",
    Data:      []byte("someBODY once told me"),
})
if err != nil {
    log.Fatal("create task driver", err)
}

worker := gosk.NewWorker[myTask, int64](pgdriver, time.Minute*5, time.Second*5)

err = worker.GetTask(ctx, gosk.Fifo, func(task myTask) (err error) {
    // Do something with the task

    if taskIsInvalid() {
        return gosk.ErrCancelTask
    }

    return nil
})
if err != nil {
    log.Fatal("error happened while working on tasks", err)
}

```

### Task lifecycle

- A task is created.
- A task is picked up by a worker.
- The worker processes the task, while this is happening the task is being
 marked as taken via a series of recurring _pings_.
- The task is finalised. If the task was correctly processed, it can be marked
as completed, otherwise it can be either released for other workers to pick it
up or can be marked as cancelled.
