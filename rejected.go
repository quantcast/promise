package promise

type RejectedPromise struct {
	cause error
}

// Create a new pure promise which has already been rejected. All calls to
// Then() and Combine() simply return this promise without executing any
// further code, and calls to Catch() invoke the handler immediately with the
// suppilied cause. Not supplying a cause is considered an illegal state.
func Rejected(cause error) Thenable {
	if cause == nil {
		panic("Rejected promise requires a non-nil cause")
	}

	promise := new(RejectedPromise)

	promise.cause = cause

	return promise
}

func (promise *RejectedPromise) Resolved() bool {
	return false
}

func (promise *RejectedPromise) Rejected() bool {
	return true
}

func (promise *RejectedPromise) Get() (interface{}, error) {
	return nil, promise.cause
}

func (promise *RejectedPromise) Then(compute func(interface{}) interface{}) Thenable {
	return promise
}

func (promise *RejectedPromise) Combine(compute func(interface{}) Thenable) Thenable {
	return promise
}

func (promise *RejectedPromise) Catch(handle func(error)) Thenable {
	handle(promise.cause)

	return promise
}
