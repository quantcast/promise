package promise

type RejectedPromise struct {
	cause error
}

func Rejected(cause error) Thenable {
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

func (promise *RejectedPromise) Get() interface{} {
	return nil
}

func (promise *RejectedPromise) Cause() error {
	return promise.cause
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
