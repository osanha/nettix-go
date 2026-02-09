package util

import (
	"sync"
	"time"
)

// ScheduledTask represents a task that is executed at scheduled intervals.
type ScheduledTask[K any] func(key K)

// ScheduledExecutor handles periodic task execution.
type ScheduledExecutor[K comparable] struct {
	initDelay time.Duration
	interval  time.Duration
	tasks     sync.Map // map[K]*scheduledFuture
	stopCh    chan struct{}
}

type scheduledFuture struct {
	cancel func()
}

// NewScheduledExecutor creates a new ScheduledExecutor.
func NewScheduledExecutor[K comparable]() *ScheduledExecutor[K] {
	return &ScheduledExecutor[K]{
		stopCh: make(chan struct{}),
	}
}

// NewScheduledExecutorWithDefaults creates a new ScheduledExecutor with default timing.
func NewScheduledExecutorWithDefaults[K comparable](initDelay, interval time.Duration) *ScheduledExecutor[K] {
	return &ScheduledExecutor[K]{
		initDelay: initDelay,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// ScheduleAtFixedRate schedules a task to run at fixed intervals.
func (e *ScheduledExecutor[K]) ScheduleAtFixedRate(key K, task ScheduledTask[K]) {
	e.ScheduleAtFixedRateWithTiming(key, task, e.initDelay, e.interval)
}

// ScheduleAtFixedRateWithTiming schedules a task with specific timing.
func (e *ScheduledExecutor[K]) ScheduleAtFixedRateWithTiming(key K, task ScheduledTask[K], initDelay, interval time.Duration) {
	e.schedule(key, task, initDelay, interval, true)
}

// ScheduleWithFixedDelay schedules a task with a fixed delay between executions.
func (e *ScheduledExecutor[K]) ScheduleWithFixedDelay(key K, task ScheduledTask[K]) {
	e.ScheduleWithFixedDelayTiming(key, task, e.initDelay, e.interval)
}

// ScheduleWithFixedDelayTiming schedules a task with specific timing.
func (e *ScheduledExecutor[K]) ScheduleWithFixedDelayTiming(key K, task ScheduledTask[K], initDelay, interval time.Duration) {
	e.schedule(key, task, initDelay, interval, false)
}

func (e *ScheduledExecutor[K]) schedule(key K, task ScheduledTask[K], initDelay, interval time.Duration, isRate bool) {
	if interval <= 0 {
		panic("interval must be positive")
	}

	stopCh := make(chan struct{})
	future := &scheduledFuture{
		cancel: func() { close(stopCh) },
	}
	e.tasks.Store(key, future)

	go func() {
		// Initial delay
		select {
		case <-time.After(initDelay):
		case <-stopCh:
			return
		case <-e.stopCh:
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			task(key)

			select {
			case <-ticker.C:
			case <-stopCh:
				return
			case <-e.stopCh:
				return
			}
		}
	}()
}

// Cancel cancels a scheduled task.
func (e *ScheduledExecutor[K]) Cancel(key K) {
	if f, ok := e.tasks.LoadAndDelete(key); ok {
		f.(*scheduledFuture).cancel()
	}
}

// Shutdown shuts down the executor.
func (e *ScheduledExecutor[K]) Shutdown() {
	close(e.stopCh)
}
