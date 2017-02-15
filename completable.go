package promise

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Unfortunately there are no atomic operations smaller values than 32
const (
	PENDING uint32 = iota
	FULFILLED
	REJECTED
)

type CompletablePromise struct {
	state        uint32
	cause        error
	value        interface{}
	mutex        sync.Mutex
	cond         *sync.Cond
	compute      func(interface{}) interface{}
	handle       func(error)
	dependencies []Completable
}

func completable(compute func(interface{}) interface{}, handle func(error)) *CompletablePromise {
	completable := new(CompletablePromise)

	completable.cond = sync.NewCond(&completable.mutex)
	completable.compute = compute
	completable.handle = handle
	completable.state = PENDING
	completable.dependencies = make([]Completable, 0)

	return completable
}

// Generate a new completable promise. This provides an implementation of the
// `promise.Completable` interface which is threadsafe.
func Promise() Completable {
	return completable(func(x interface{}) interface{} { return x }, nil)
}

func (promise *CompletablePromise) State() uint32 {
	return atomic.LoadUint32(&promise.state)
}

// Determine if the promise has been resolved.
func (promise *CompletablePromise) Resolved() bool {
	return promise.State() == FULFILLED
}

func (promise *CompletablePromise) Rejected() bool {
	return promise.State() == REJECTED
}

// Return the value of the promise, if it was resolved successfully, or return
// the cause of failure if it was not. Block until the promise is either
// completed or rejected.
func (promise *CompletablePromise) Get() (interface{}, error) {
	if promise.State() == PENDING {
		promise.mutex.Lock()

		for promise.State() == PENDING {
			// wait unlocks its associated mutex (incase you were wondering)
			// so we cannot guarantee that the state has actually changed.
			promise.cond.Wait()
		}

		promise.mutex.Unlock()
	}

	return promise.value, promise.cause
}

func (promise *CompletablePromise) depend(compute func(interface{}) interface{}) Thenable {
	andThen := completable(compute, nil)

	promise.dependencies = append(promise.dependencies, andThen)

	return andThen
}

// The private version of this is used for `Combine` to call, so that it won't
// attempt to acquire the mutex twice.
func (promise *CompletablePromise) then(compute func(interface{}) interface{}) Thenable {
	switch promise.State() {
	case PENDING:
		return promise.depend(compute)
	case REJECTED:
		return Rejected(promise.cause)
	case FULFILLED:
		return Completed(compute(promise.value))
	}

	panic("Invalid state")
}

// Compose this promise into one which is complete when the following code has
// executed.
func (promise *CompletablePromise) Then(compute func(interface{}) interface{}) Thenable {
	switch promise.State() {
	case PENDING:
		promise.mutex.Lock()

		defer promise.mutex.Unlock()

		return promise.then(compute)
	case REJECTED:
		return Rejected(promise.cause)
	case FULFILLED:
		return Completed(compute(promise.value))
	}

	panic("Invalid state")
}

// Compose this promise into another one which handles an upstream error with
// the given handler.
func (promise *CompletablePromise) Catch(handle func(error)) Thenable {
	if promise.State() == PENDING {
		promise.mutex.Lock()

		defer promise.mutex.Unlock()

		// Double check now that we have the lock that this is still true.
		if promise.State() == PENDING {
			rejectable := completable(nil, handle)

			promise.dependencies = append(promise.dependencies, rejectable)

			return rejectable
		}
	}

	if promise.State() == REJECTED {
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

func (promise *CompletablePromise) complete(value interface{}) interface{} {
	// This should rarely actually be blocking, there's a separate mutex for
	// each completable promise and the mutex is only acquired during assembly
	// and completion.
	promise.mutex.Lock()

	defer promise.mutex.Unlock()

	composed := value

	if promise.compute != nil {
		// Because this composition function
		composed = promise.compute(value)
	}

	if promise.State() != PENDING {
		panicStateComplete(promise.State() == REJECTED)
	}

	if composed != nil {
		promise.value = composed
	}

	atomic.StoreUint32(&promise.state, FULFILLED)

	return composed
}

// Complete this promise with a given value.
// It is considered a programming error to complete a promise multiple times.
// The promise is to be completed once, and not thereafter.
func (promise *CompletablePromise) Complete(value interface{}) {
	// Transition the state of this promise (which requires the lock). At this
	// point all subsequent calls to Then() or Complete() will be called on a
	// Completed promise, meaning they will be satisfied immediately.
	composed := promise.complete(value)

	// So now that the condition has been satisified, broadcast to all waiters
	// that thie task is now complete. They should be in the `Get()` wait loop,
	// above.
	promise.cond.Broadcast()

	for _, dependency := range promise.dependencies {
		dependency.Complete(composed)
	}
}

// Reject this promise and all of its dependencies.
// Reject this promise, and along with it all promises which were derived from
// it.
func (promise *CompletablePromise) Reject(cause error) {
	if cause == nil {
		panic(fmt.Sprintf("Reject() requires a non-nil cause"))
	}

	promise.mutex.Lock()

	if promise.State() != PENDING {
		panicStateComplete(promise.State() == REJECTED)
	}

	promise.cause = cause

	atomic.StoreUint32(&promise.state, REJECTED)

	promise.mutex.Unlock()

	// Unlike the Complete() routine, which executes the transformation
	// *before* actually storing the value or transitioning the state, this
	// transitions after. The reason for that is two-fold: The return value of
	// the handle() callback is not *stored*, and the second is that we want a
	// promise accessed from within the Catch() handler to be in a rejected
	// state.
	if promise.handle != nil {
		promise.handle(cause)
	}

	// Now that this is all done, notify all of the handlers that yeah, we're
	// done.
	promise.cond.Broadcast()

	for _, dependency := range promise.dependencies {
		dependency.Reject(cause)
	}
}

// Combine this promise with another by applying the combinator `create` to the
// value once it is available. `create` must return an instance of a
// `Thenable`. The instance *may* be `Completable`. Returns a new completable
// promise which is completed when the returned promise, and this promise, are
// completed...but no sooner.
func (promise *CompletablePromise) Combine(create func(interface{}) Thenable) Thenable {
	if promise.State() == PENDING {
		promise.mutex.Lock()

		defer promise.mutex.Unlock()

		if promise.State() == PENDING {
			// So, this may seem a little whacky, but what is happening here is
			// that seeing as there is presently no value from which to generate
			// the new promise, a callback is registered using Then() which
			// executes the supplied transform function, and when the promise that
			// was returned by *that* transform produces a result, it is copied
			// over to the placeholder thus satisfying the request.
			placeholder := Promise()

			// So, is it possible that Combine() is called, and the promise is
			// completed while it's being combined? Should *not* be.
			//
			// Perhaps all access to promise.state should be atomic. We are
			// using the double lock idiom here, after all...

			// It's important that the internal then() is used here, because the
			// external one allocates a mutex lock. sync.Mutex is not a reentrant lock
			// type, unfortunately.
			promise.depend(func(awaited interface{}) interface{} {
				create(awaited).Then(func(composed interface{}) interface{} {
					placeholder.Complete(composed)

					return nil
				}).Catch(func(err error) {
					placeholder.Reject(err)
				})

				return nil
			}).Catch(func(err error) {
				placeholder.Reject(err)
			})

			return placeholder
		}
	}

	if promise.state == REJECTED {
		return Rejected(promise.cause)
	} else {
		return create(promise.value)
	}
}
