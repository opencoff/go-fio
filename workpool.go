// workpool.go - worker pool abstraction
//
// Workers are per-cpu go-routines that accept work submitted
// via a channel and invokes a caller defined "work" function.
//
// The API is modeled after sync.WaitGroup; a typical invocation
// looks like so:
//
//
//	// this is a unit of work to be performed
//	type myWork struct {
//	    ...
//	}
//
//	pool := NewWorkPool[myWork](func(cpunum int, w myWork) error {
//		.. process the work here
//		.. return error as needed
//		return nil
//		})
//
//
//	....
//
//	// submit work in a different go-routine
//	pool.Submit(work)
//
//	...
//	// close the pool after there is no more work to submit
//	pool.Close()
//
//	...
//	// wait for pool to complete the work
//	// captures all the errors returned by the workers
//	err := pool.Wait()
//
//  Wait() harvests the errors and closes all the worker goroutines.
//  Thus, the pool cannot be used to submit new work after Wait()
//  is called.

package fio

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

type WorkPool[Work any] struct {
	stopped atomic.Bool
	wg      sync.WaitGroup
	ch      chan Work

	ech  chan error
	ewg  sync.WaitGroup
	errs []error
}

// Error returned if new work is submitted after Wait() or if Wait() is called
// multiple times.
var ErrCompleted = errors.New("workpool: workpool closed")

// Error returned if Wait() is called without closing the work submission
var ErrNotClosed = errors.New("workpool: workpool not closed before waiting")

// NewWorkPool creates a worker pool that invokes caller provided worker 'fp'.
// Each worker will process one unit of "work" submitted via Submit().
func NewWorkPool[Work any](nworkers int, fp func(i int, w Work) error) *WorkPool[Work] {
	if nworkers <= 1 {
		nworkers = runtime.NumCPU()
	}

	wp := &WorkPool[Work]{
		ch:   make(chan Work, nworkers),
		ech:  make(chan error, 1),
		errs: make([]error, 0, 1),
	}

	wp.stopped.Store(false)
	wp.wg.Add(nworkers)
	for i := 0; i < nworkers; i++ {
		go func(i int, fp func(i int, w Work) error) {
			defer func() {
				if e := recover(); e != nil {
					if err := e.(error); err != nil {
						wp.ech <- fmt.Errorf("workpool: panic: %w", err)
					}
				}
			}()

			for w := range wp.ch {
				err := fp(i, w)
				if err != nil {
					wp.ech <- err
				}
			}
			wp.wg.Done()
		}(i, fp)
	}

	// harvest errors
	wp.ewg.Add(1)
	go func(ech chan error) {
		for e := range wp.ech {
			wp.errs = append(wp.errs, e)
		}
		wp.ewg.Done()
	}(wp.ech)

	return wp
}

// Wait closes the work channel and waits for all workers
// to end. Returns any errors from the workers.
// It is an error to call this multiple times
func (wp *WorkPool[Work]) Wait() error {
	wp.wg.Wait()
	close(wp.ech)

	// wait for error harvestor to complete
	wp.ewg.Wait()
	if len(wp.errs) > 0 {
		return errors.Join(wp.errs...)
	}
	return nil
}

// Close the work submission to workers and signal
// to them that there's no more work forthcoming.
func (wp *WorkPool[Work]) Close() {
	if wp.stopped.Swap(true) {
		panic("worker already closed")
	}
	close(wp.ch)
}

// Submit submits one unit of work to the worker
// WorkPool must be active.
func (wp *WorkPool[Work]) Submit(w Work) {
	if wp.stopped.Load() {
		panic("worker stopped")
	}
	wp.ch <- w
}

// Submit an error to the pool - if the user provided
// worker does things asynchronously.
func (wp *WorkPool[Work]) Err(err error) {
	if !wp.stopped.Load() {
		wp.ech <- err
	}
}
