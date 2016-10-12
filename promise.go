// Promises for golang.
//
// This package implements a set of concurrency primitives for composing
// executions upon values which may exist in the future. They follow the model
// of JavaScript's Promises/A+ (https://promisesaplus.com/), with some
// exceptions centric to Go. There are both applicative functor (`Then()`) and
// monadic combinator (`Combine()`) methods.
package promise

// A computation which can be composed with Then().
// Types which implement this interface can be composed with the Then() method,
// they have an indicator of their status, Resolved(), which determines whether
// or not calls to the Get() method are safe. Otherwise, the presence of a
// value is observable with Then().
type Thenable interface {

	// Resolved determines whether or not calling Get() is safe.
	Resolved() bool

	// Errored determines whether or not a promise has been rejected at any
	// point.
	Rejected() bool

	// Create a new Thenable which is the result of this computation and the
	// transformation function herein.
	// TODO This needs a way to actually give you a new promise.
	Then(func(interface{}) interface{}) Thenable

	// Combine this thenable with another thenable.
	// Given a function which accepts the value from this thenable, return a
	// new Thenable that is resolved with the Thenable that is the result of
	// the given function.
	Combine(func(interface{}) Thenable) Thenable

	// Handle an error which has occurred during processing.
	Catch(func(error)) Thenable

	// Return the value of this Thenable, or the error which occurred.
	// Implementations which are impure must block until the promise is either
	// resolved or rejected.
	Get() (interface{}, error)
}

// A promise which can be completed.
type Completable interface {
	Thenable

	// Complete a promise.
	Complete(interface{})

	// Reject this promise and all of its derivatives.
	Reject(error)
}

// Combine the given promises as a single promise which produces a slice of
// values. Given an arbitrarily long list of promises (as variadic arguments)
// combine all of the promises to a single promise which transforms all of the
// results of the promises into a slice (as []interface{}).
func All(thenables ...Thenable) Thenable {
	var cursor Thenable

	for _, each := range thenables {
		// For the first thenable, transform it's value into an array of
		// values.
		if cursor == nil {
			cursor = each.Then(func(value interface{}) interface{} {
				return []interface{}{value}
			})

			continue
		}

		// Afterward, we combine that promise and the next one, but in order to
		// maintain referential integrity given there's no guarantees about
		// when the combine function runs, we have to close over the value so
		// as to copy the reference.
		cursor = func(promise Thenable) Thenable {
			return cursor.Combine(func(left interface{}) Thenable {
				values, ok := left.([]interface{})

				if !ok {
					panic("Expected an array in combiner")
				}

				return promise.Then(func(right interface{}) interface{} {
					return append(values, right)
				})
			})
		}(each)
	}

	return cursor
}
