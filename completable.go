package promise

import (
	"fmt"
	"sync"
)

type CompletablePromise struct {
	completed    bool
	rejected     bool
	cause        error
	value        interface{}
	mutex        sync.Mutex
	waitGroup    sync.WaitGroup
	compute      func(interface{}) interface{}
	handle       func(error)
	dependencies []Completable
}

func completable(compute func(interface{}) interface{}, handle func(error)) *CompletablePromise {
	completable := new(CompletablePromise)

	completable.compute = compute
	completable.handle = handle
	completable.completed = false
	completable.rejected = false
	completable.dependencies = make([]Completable, 0)
	completable.waitGroup.Add(1)

	return completable
}

// Generate a new completable promise. This provides an implementation of the
// `promise.Completable` interface which is threadsafe.
func Promise() Completable {
	return completable(func(x interface{}) interface{} { return x }, nil)
}

// Determine if the promise has been resolved.
func (promise *CompletablePromise) Resolved() bool {
	return promise.completed && !promise.rejected
}

func (promise *CompletablePromise) Rejected() bool {
	return promise.rejected
}

// Return the value of the promise, if it was resolved successfully, or return
// the cause of failure if it was not. Block until the promise is either
// completed or rejected.
func (promise *CompletablePromise) Get() (interface{}, error) {
	if !promise.completed {
		promise.waitGroup.Wait()
	}

	return promise.value, promise.cause
}

func (promise *CompletablePromise) Cause() error {
	return promise.cause
}

// The private version of this is used for `Combine` to call, so that it won't
// attempt to acquire the mutex twice.
func (promise *CompletablePromise) then(compute func(interface{}) interface{}) Thenable {
	if !promise.completed {
		andThen := completable(compute, nil)

		promise.dependencies = append(promise.dependencies, andThen)

		return andThen
	} else if promise.rejected {
		return Rejected(promise.cause)
	} else {
		return Completed(compute(promise.value))
	}
}

// Compose this promise into one which is complete when the following code has
// executed.
func (promise *CompletablePromise) Then(compute func(interface{}) interface{}) Thenable {
	if !promise.completed {
		promise.mutex.Lock()

		defer promise.mutex.Unlock()

		return promise.then(compute)
	} else if promise.rejected {
		return Rejected(promise.cause)
	} else {
		return Completed(compute(promise.value))
	}
}

// Compose this promise into another one which handles an upstream error with
// the given handler.
func (promise *CompletablePromise) Catch(handle func(error)) Thenable {
	if !promise.completed {
		promise.mutex.Lock()

		defer promise.mutex.Unlock()

		// Double check now that we have the lock that this is still true.
		if !promise.completed {
			rejectable := completable(nil, handle)

			promise.dependencies = append(promise.dependencies, rejectable)

			return rejectable
		}
	}

	if promise.rejected {
		handle(promise.cause)

		return Rejected(promise.cause)
	}

	return promise
}

// Error due to an illegal second state transition, after figuring out what
// caused the previous state transition.
func panicStateComplete(rejected bool) {
	var method string

	if rejected {
		method = "Reject()"
	} else {
		method = "Complete()"
	}

	panic(fmt.Sprintf("%s was already called on this promise", method))
}

// Complete this promise with a given value.
// It is considered a programming error to complete a promise multiple times.
// The promise is to be completed once, and not thereafter.
func (promise *CompletablePromise) Complete(value interface{}) {
	// This should rarely actually be blocking, there's a separate mutex for
	// each completable promise and the mutex is only acquired during assembly
	// and completion.
	promise.mutex.Lock()

	defer promise.mutex.Unlock()

	if promise.completed {
		panicStateComplete(promise.rejected)
	}

	composed := value

	if promise.compute != nil {
		composed = promise.compute(value)
	}

	if composed != nil {
		promise.value = composed
	}

	promise.waitGroup.Done()

	for _, dependency := range promise.dependencies {
		dependency.Complete(composed)
	}

	promise.completed = true
}

// Reject this promise and all of its dependencies.
// Reject this promise, and along with it all promises which were derived from
// it.
func (promise *CompletablePromise) Reject(cause error) {
	if cause == nil {
		panic(fmt.Sprintf("Reject() requires a non-nil cause"))
	}

	promise.mutex.Lock()

	defer promise.mutex.Unlock()

	if promise.completed {
		panicStateComplete(promise.rejected)
	}

	if promise.handle != nil {
		promise.handle(cause)
	}

	promise.waitGroup.Done()

	for _, dependency := range promise.dependencies {
		dependency.Reject(cause)
	}

	promise.completed = true
	promise.rejected = true
}

// Combine this promise with another by applying the combinator `create` to the
// value once it is available. `create` must return an instance of a
// `Thenable`. The instance *may* be `Completable`. Returns a new completable
// promise which is completed when the returned promise, and this promise, are
// completed...but no sooner.
func (promise *CompletablePromise) Combine(create func(interface{}) Thenable) Thenable {
	if !promise.completed {
		promise.mutex.Lock()

		defer promise.mutex.Unlock()

		if !promise.completed {
			// So, this may seem a little whacky, but what is happening here is
			// that seeing as there is presently no value from which to generate
			// the new promise, a callback is registered using Then() which
			// executes the supplied transform function, and when the promise that
			// was returned by *that* transform produces a result, it is copied
			// over to the placeholder thus satisfying the request.
			placeholder := Promise()

			// It's important that the internal then() is used here, because the
			// external one allocates a mutex lock. sync.Mutex is not a reentrant lock
			// type, unfortunately.
			promise.then(func(awaited interface{}) interface{} {
				create(awaited).Then(func(composed interface{}) interface{} {
					placeholder.Complete(composed)

					return nil
				})

				return nil
			})

			return placeholder
		}
	}

	if promise.rejected {
		return Rejected(promise.cause)
	} else {
		return create(promise.value)
	}
}
