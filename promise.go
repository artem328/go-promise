// Package promise contains implementation of promises for GoLang
package promise

import (
	"errors"
	"fmt"
	"sync"
)

// States of promise
const (
	Fulfilled State = "fulfilled"
	Rejected  State = "rejected"
)

// Predefined errors
var (
	ErrAllPromisesRejected = errors.New("all promises rejected")
	ErrNoRacers            = errors.New("no racers")
)

type (
	// State represents a promise's state
	State string

	// Promise is the state struct for a single promise.
	// Use promise constructors to create a promise.
	Promise[T any] struct {
		val T
		err error

		done chan struct{}

		once sync.Once
	}
)

// New calls executor function asynchronously and returns promise instance
// holding the state of the executor.
//
// Either resolve or reject must be called in the executor function to avoid goroutine leak.
//
// If executor panics the promise will be rejected.
func New[T any](executor func(resolve func(T), reject func(error))) *Promise[T] {
	p := newPromise[T]()

	go func() {
		defer func() {
			e := recover()
			if e == nil {
				return
			}

			switch err := e.(type) {
			case error:
				p.reject(fmt.Errorf("panic: %w", err))
			case string:
				p.reject(fmt.Errorf("panic: %s", err))
			case fmt.Stringer:
				p.reject(fmt.Errorf("panic: %s", err.String()))
			default:
				p.reject(fmt.Errorf("panic: %#v", err))
			}
		}()
		executor(p.resolve, p.reject)
	}()

	return p
}

// WithResolvers returns a new instance of promise and resolve and reject functions
// to settle the returned promise.
func WithResolvers[T any]() (promise *Promise[T], resolve func(T), reject func(error)) {
	p := newPromise[T]()

	return p, p.resolve, p.reject
}

// All returns the promise that fulfills with values from all the passed promises
// when all of them are fulfilled.
//
// In case any passed promise is rejected the returned promise rejects with error from
// the rejected promise.
//
// When no promises are provided the fulfilled promise is returned instantly with an empty value.
func All[T any](promises ...*Promise[T]) *Promise[[]T] {
	var results []T

	if len(promises) == 0 {
		return Resolve(results)
	}

	return New(func(resolve func([]T), reject func(error)) {
		valCh := make(chan result[T], len(promises))
		errCh := make(chan error, 1)
		defer drain(valCh)
		defer drain(errCh)
		done := make(chan struct{})
		defer close(done)

		for i := 0; i < len(promises); i++ {
			go func(i int) {
				v, err := promises[i].Await()
				if err != nil {
					select {
					case <-done:
					case errCh <- err:
					}

					return
				}

				select {
				case <-done:
				case valCh <- result[T]{i: i, val: v}:
				}
			}(i)
		}

		results = make([]T, len(promises))
		for i := 0; i < len(promises); i++ {
			select {
			case r := <-valCh:
				results[r.i] = r.val
			case err := <-errCh:
				reject(err)
				return
			}
		}

		resolve(results)
	})
}

// AllSettled returns the promise that always fulfills with all the states from passed promises
// when they are settled.
//
// When no promises are provided the fulfilled promise is returned instantly with an empty value.
func AllSettled[T any](promises ...*Promise[T]) *Promise[[]State] {
	var results []State
	if len(promises) == 0 {
		return Resolve(results)
	}

	return New(func(resolve func([]State), reject func(error)) {
		resCh := make(chan result[State], len(promises))

		for i := 0; i < len(promises); i++ {
			go func(i int) {
				_, err := promises[i].Await()
				if err != nil {
					resCh <- result[State]{i: i, val: Rejected}
					return
				}

				resCh <- result[State]{i: i, val: Fulfilled}
			}(i)
		}

		results = make([]State, len(promises))
		for i := 0; i < len(promises); i++ {
			r := <-resCh
			results[r.i] = r.val
		}

		resolve(results)
	})
}

// Race returns the promise that settles whenever the first of the passed promises is settled,
// whether with success or error.
//
// When no promises are provided the rejected promise is returned instantly with the [ErrNoRacers] error,
func Race[T any](promises ...*Promise[T]) *Promise[T] {
	if len(promises) == 0 {
		return Reject[T](ErrNoRacers)
	}

	return New(func(resolve func(T), reject func(error)) {
		valCh := make(chan T, 1)
		errCh := make(chan error, 1)
		drain(valCh)
		drain(errCh)
		done := make(chan struct{})
		defer close(done)

		for i := 0; i < len(promises); i++ {
			go func(i int) {
				v, err := promises[i].Await()
				if err != nil {
					select {
					case <-done:
					case errCh <- err:
					}

					return
				}

				select {
				case <-done:
				case valCh <- v:
				}
			}(i)
		}

		select {
		case v := <-valCh:
			resolve(v)
		case err := <-errCh:
			reject(err)
		}
	})
}

// Any returns the promise that is fulfilled whenever the first of the passed promises fulfills.
//
// In case all the passed promises are rejected the returned promise is rejected
// with the [ErrAllPromisesRejected] error.
//
// When no promises are provided the rejected promise is returned instantly with the [ErrAllPromisesRejected] error.
func Any[T any](promises ...*Promise[T]) *Promise[T] {
	if len(promises) == 0 {
		return Reject[T](ErrAllPromisesRejected)
	}

	return New(func(resolve func(T), reject func(error)) {
		valCh := make(chan T, 1)
		errCh := make(chan struct{}, len(promises))
		defer drain(valCh)
		defer drain(errCh)
		done := make(chan struct{})
		defer close(done)

		for i := 0; i < len(promises); i++ {
			go func(i int) {
				v, err := promises[i].Await()
				if err != nil {
					select {
					case <-done:
					case errCh <- struct{}{}:
					}

					return
				}

				select {
				case <-done:
				case valCh <- v:
				}
			}(i)
		}

		for i := 0; i < len(promises); i++ {
			select {
			case v := <-valCh:
				resolve(v)
				return
			case <-errCh:
			}
		}

		reject(ErrAllPromisesRejected)
	})
}

// Resolve returns fulfilled promise with value provided as argument.
func Resolve[T any](v T) *Promise[T] {
	p := newPromise[T]()
	p.val = v
	close(p.done)

	return p
}

// Reject returns rejected promise with error provided as argument.
func Reject[T any](err error) *Promise[T] {
	p := newPromise[T]()
	p.err = err
	close(p.done)

	return p
}

// Then returns a new equivalent promise instance that will call onFulfilled function when the current
// promise is fulfilled before fulfilling the returned promise.
//
// The onFulfilled function can return another promise to provide promise chaining. In this case
// the returned promise will settle after the returned promise from onFulfilled is settled.
//
//	var p1 *Promise[int]
//	p2 := p1.Then(func(i int) *Promise[int] {
//		return Promise.resolve(i * 2)
//	}) // p2 will be settled after Promise.resolve(i * 2) is settled
//
// If the onFulfilled function returns nil
// the returned promise is fulfilled right after the onFulfilled func is finished.
//
//	var p1 *Promise[int]
//	p2 := p1.Then(func(i int) *Promise[int] {
//		doSomething()
//		return nil
//	}) // p2 will be fulfilled after doSomething() is finished and nil is returned.
func (p *Promise[T]) Then(onFulfilled func(T) *Promise[T]) *Promise[T] {
	return p.then(onFulfilled, nil)
}

// Catch returns a new equivalent promise instance that will call onRejected function when the current
// promise is rejected before rejecting the returned promise.
//
// The onRejected function can return another promise to provide promise chaining. In this case
// the returned promise will settle after the returned promise from onRejected is settled.
//
//	var p1 *Promise[int]
//	p2 := p1.Catch(func(err error) *Promise[int] {
//		return Promise.resolve("caught")
//	}) // p2 will be settled after Promise.resolve("caught") is settled
//
// If the onRejected function returns nil the returned promise is rejected
// right after the onRejected func is finished.
//
//	var p1 *Promise[int]
//	p2 := p1.Catch(func(err error) *Promise[int] {
//		doSomething()
//		return nil
//	}) // p2 will be rejected after doSomething() is finished and nil is returned.
func (p *Promise[T]) Catch(onRejected func(error) *Promise[T]) *Promise[T] {
	return New(func(resolve func(T), reject func(error)) {
		v, err := p.Await()
		if err == nil {
			resolve(v)
			return
		} else if onRejected == nil {
			reject(err)
			return
		}

		next := onRejected(err)
		if next == nil {
			reject(err)
			return
		}

		v, err = next.Await()
		if err != nil {
			reject(err)
			return
		}
		resolve(v)
	})
}

// ThenCatch is similar to [Promise.Then] except the fact that it also accepts onRejected callback,
// which is called in case the promise is rejected.
// The onRejected function can return a new promise. See more details in [Promise.Catch].
//
// Note: p.Then(onFulfilled).Catch(onRejected) chain differs from ThenCatch since it produces only one promise,
// thus only one goroutine. While each chain call produces a new promise with dedicated goroutine, which means
// p.Then(onFulfilled).Catch(onRejected) will spawn two goroutines.
func (p *Promise[T]) ThenCatch(onFulfilled func(T) *Promise[T], onRejected func(error) *Promise[T]) *Promise[T] {
	return p.then(onFulfilled, onRejected)
}

// Finally returns a new equivalent promise instance that will call onFinally function when the current
// promise is settled regardless of its state before settling the returned promise.
//
// The onFinally function can return another promise to provide promise chaining. In this case
// the returned promise will settle after the returned promise from onFinally is settled.
//
//	var p1 *Promise[int]
//	p2 := p1.Finally(func(err error) *Promise[int] {
//		return Promise.resolve("caught")
//	}) // p2 will be settled after Promise.resolve("caught") is settled
//
// If the onFinally function returns nil the returned promise is settled
// right after the onFinally func is finished.
//
//	var p1 *Promise[int]
//	p2 := p1.Finally(func(err error) *Promise[int] {
//		doSomething()
//		return nil
//	}) // p2 will be settled after doSomething() is finished and nil is returned.
func (p *Promise[T]) Finally(onFinally func() *Promise[T]) *Promise[T] {
	return New(func(resolve func(T), reject func(error)) {
		v, err := p.Await()

		if onFinally != nil {
			next := onFinally()
			if next != nil {
				v, err = next.Await()
			}
		}

		if err != nil {
			reject(err)
			return
		}

		resolve(v)
	})
}

// Await waits until the promise is settled and returns settlement value and error.
// If the promise is already settled further calls to Await won't block the execution.
// The error will be a non-nil value if promise is rejected.
//
// The method will always return the same result after the promise is settled.
//
// To test if Await could block the execution the [Promise.Settled] can be used.
func (p *Promise[T]) Await() (T, error) {
	<-p.done
	return p.val, p.err
}

// Settled returns channel to test if the promise was settled.
// When the channel can be read from, it means that the promise is settled and further calls to [Promise.Await]
// will not block.
//
//	var p *Promise[any]
//	select {
//	case <-p.Settled():
//		// do something (e.g. read the settlement result)
//		val, err := p.Await() // Await() will return instantly since we already know that the promise is settled
//	default:
//		// do something else
//	}
func (p *Promise[T]) Settled() <-chan struct{} {
	return p.done
}

func (p *Promise[T]) then(onFulfilled func(T) *Promise[T], onRejected func(error) *Promise[T]) *Promise[T] {
	return New(func(resolve func(T), reject func(error)) {
		var next *Promise[T]

		v, err := p.Await()
		switch err {
		case nil:
			if onFulfilled == nil {
				resolve(v)
				return
			}

			next = onFulfilled(v)
			if next == nil {
				resolve(v)
				return
			}
		default:
			if onRejected == nil {
				reject(err)
				return
			}

			next = onRejected(err)
			if next == nil {
				reject(err)
				return
			}
		}

		v, err = next.Await()
		if err != nil {
			reject(err)
			return
		}
		resolve(v)
	})
}

func (p *Promise[T]) resolve(val T) {
	p.once.Do(func() {
		p.val = val
		close(p.done)
	})
}

func (p *Promise[T]) reject(err error) {
	p.once.Do(func() {
		p.err = err
		close(p.done)
	})
}

type result[T any] struct {
	i   int
	val T
}

func drain[T any](c <-chan T) {
	for len(c) > 0 {
		<-c
	}
}

func newPromise[T any]() *Promise[T] {
	done := make(chan struct{})

	return &Promise[T]{done: done}
}
