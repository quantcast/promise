package promise

// A completed promise. as returned by the `promise.Completed()` method.
type CompletedPromise struct {
	value interface{}
}

// Create a new completed promise (with a given value). Given a value, a
// completed promise is returned, the completed promise has a `Resolved()`
// value of `true`, and `Then()` and `Combine()` execute immediately.
func Completed(value interface{}) Thenable {
	completed := new(CompletedPromise)

	completed.value = value

	return completed
}

// Always true, a completed promise is completed from initialization.
func (promise *CompletedPromise) Resolved() bool {
	return true
}

func (promise *CompletedPromise) Rejected() bool {
	return false
}

// Always returns the value that this promise was initialized with.
func (promise *CompletedPromise) Get() interface{} {
	return promise.value
}

func (promise *CompletedPromise) Cause() error {
	return nil
}

// Create a completed promise for the value of this promise with the compute
// function applied.
func (promise *CompletedPromise) Then(compute func(interface{}) interface{}) Thenable {
	return Completed(compute(promise.value))
}

// Create a promise from this value and another promise.
func (promise *CompletedPromise) Combine(create func(interface{}) Thenable) Thenable {
	return create(promise.value)
}

func (promise *CompletedPromise) Catch(handle func(error)) Thenable {
	return promise
}
