package promise

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// Ensure that the basic properties of a promise holds true if the value is
// already resolved.
func TestCompletedPromise(test *testing.T) {
	value, _ := Completed(10).Then(func(foo interface{}) interface{} {
		i, ok := foo.(int)

		if !ok {
			test.Fatalf("Expected type int")
		}

		return i + 10
	}).Then(func(foo interface{}) interface{} {
		i, ok := foo.(int)

		if !ok {
			test.Fatalf("Expected type int")
		}

		if i != 20 {
			test.Fatalf("Expected 20, saw %d", i)
		}

		return i == 20
	}).Combine(func(foo interface{}) Thenable {
		/* Just a pass-through to say we did it */
		return Completed(foo)
	}).Get()

	result, ok := value.(bool)

	if !ok {
		test.Fatalf("Expected boolean result from .Get()")
	}

	if result != true {
		test.Fatalf("Expected result to be true!")
	}
}

// Validate that a promise that depends on a promise may use that promise when
// it is completed.
func TestReentrantComplete(test *testing.T) {
	go func() {
		time.Sleep(5 * time.Second)
		test.Fatalf("TestReentrantComplete appears to have deadlocked")
	}()

	a := Promise()
	b := Promise()

	b.Combine(func(value interface{}) Thenable {
		bValue, _ := value.(int)

		return a.Then(func(value interface{}) interface{} {
			aValue, _ := value.(int)

			return bValue + aValue
		})
	}).Then(func(value interface{}) interface{} {
		sumValue, _ := value.(int)

		if sumValue != 5 {
			test.Fatalf("Expected sumValue(%d) to be 5", sumValue)
		}

		return nil
	})

	// This is the strange edge case where this occurs.  For some reason
	// calling complete on promise A must cause Then() (or similar) on A to be
	// called.
	a.Then(func(value interface{}) interface{} {
		b.Complete(3)

		return nil
	})

	a.Complete(2)
}

// Ensure that the basic functions of the Promise API work for values that are
// not yet resolved.
func TestCompletablePromise(test *testing.T) {
	promise := Promise()

	squared := promise.Then(func(value interface{}) interface{} {
		val, _ := value.(int)

		return val * val
	})

	cubed := squared.Then(func(value interface{}) interface{} {
		val, _ := value.(int)

		return val * val * val
	})

	combined := promise.Combine(func(value interface{}) Thenable {
		val, _ := value.(int)

		return Completed(val + 3)
	})

	/* And then something happened...in the background! */
	go promise.Complete(2)

	squaredV, _ := squared.Get()
	combinedV, _ := combined.Get()
	cubedV, _ := cubed.Get()

	four, _ := squaredV.(int)
	five, _ := combinedV.(int)
	sixtyfour, _ := cubedV.(int)

	if four != 4 {
		test.Fatalf("Expected result of 2² to be 4")
	}

	if five != 5 {
		test.Fatalf("Expected result of 2 + 3 (%d) to be 5", five)
	}

	if sixtyfour != 64 {
		test.Fatalf("Expected result of 4³ to be 64")
	}

	thenSatisfied := false

	// This is already satisifed and *must* run immediately.
	combined.Then(func(value interface{}) interface{} {
		thenSatisfied = true

		return value
	})

	if !thenSatisfied {
		test.Fatalf("Executed completed promise.Then() to run immediately")
	}

	thenSatisfied = false

	All(squared, cubed, combined).Then(
		func(value interface{}) interface{} {
			thenSatisfied = true
			return nil
		},
	)

	if !thenSatisfied {
		test.Fatalf("Executed completed promise.All() to run immediately")
	}
}

const (
	WAITERS     = 100
	LATECHEKERS = 20
)

func TestWaitgroups(test *testing.T) {
	promise := Promise()

	var counter uint64 = 0

	waiterChan := make(chan Thenable)

	waiterPromises := make([]Thenable, 0, WAITERS)

	// Create 100 waiters...
	for i := 0; i < WAITERS; i++ {
		go (func() {
			waiterChan <- promise.Then(func(value interface{}) interface{} {
				atomic.AddUint64(&counter, 1)

				return nil
			})
		})()
	}

	go (func() {
		promise.Complete(true)
	})()

	// Wait for all of the promises created to be gathered over the channel.
	// It's important to only mutate the slice in a single goroutine, so that
	// happens here.
	for i := 0; i < WAITERS; i++ {
		waiterPromises = append(waiterPromises, <-waiterChan)
	}

	// The promises will now be in varying states of completion, observation
	// has shown ¼th of the time the final promise will be being completed
	// while this Get() method is running, causing a race condition whereby
	// Get() never returned.
	_, err := All(waiterPromises...).Get()

	if err != nil {
		test.Fatalf("Unexpected error: %s", err)
	}

	if counter != WAITERS {
		test.Fatalf("Expected a recieved count of %d, observed %d",
			WAITERS, counter)
	}

	for i := 0; i < LATECHEKERS; i++ {
		promise.Then(func(value interface{}) interface{} {
			atomic.AddUint64(&counter, 1)

			return nil
		})
	}

	if counter != LATECHEKERS+WAITERS {
		test.Fatalf("Expected a recieved count of %d, observed %d",
			LATECHEKERS+WAITERS, counter)
	}
}

// Validate that promise.All works as expected.
func TestAll(test *testing.T) {
	expected := []int{1, 2}

	res, _ := All(Completed(1), Completed(2)).Then(func(result interface{}) interface{} {
		// Traversing through these means first getting a slice of anonymous
		// values...
		values, ok := result.([]interface{})

		if !ok {
			test.Fatalf("Expected a slice of []interface{}")
		}

		// Then looking at each value...
		for i, value := range values {
			observed, ok := value.(int)

			if !ok {
				test.Fatalf("Expected int type")
			}

			if expected[i] != observed {
				test.Fatalf("Expected %d != %d observed", expected[i], observed)
			}
		}

		return true
	}).Get()

	status := res.(bool)

	if !status {
		test.Fatalf("Test cases did not run")
	}
}

// Validate that rejections on completed promises work, and Catch on rejected
// promises works as expected.
func TestRejected(test *testing.T) {
	promise := Promise()

	catchWorked := false
	dependenciesToo := false

	var expected = errors.New("Expected error!")

	promise.Catch(func(err error) {
		if err != expected {
			test.Fatalf("Did not see expected error!")
		}

		catchWorked = true
	})

	promise.Then(func(val interface{}) interface{} {
		i, _ := val.(int)

		i += 1

		return i
	}).Catch(func(err error) {
		dependenciesToo = true
	})

	alsoRejected := promise.Combine(func(val interface{}) Thenable {
		return Completed(true)
	})

	promise.Reject(expected)

	if !catchWorked {
		test.Fatalf("Did not see expected rejected handler")
	}

	if !dependenciesToo {
		test.Fatalf("Did not see rejected handler on dependency")
	}

	promise.Catch(func(cause error) {
		if cause != expected {
			test.Fatalf("previously rejected promise did not pass on the cause")
		}
	})

	alsoRejected.Catch(func(cause error) {
		if cause != expected {
			test.Fatalf("Expected combined promises to result in errors also")
		}
	})

	rejectedCalled := false

	Rejected(expected).Catch(func(err error) {
		if err != expected {
			test.Fatalf("Rejected() promise does not relay messages.")
		}

		rejectedCalled = true
	})

	if !rejectedCalled {
		test.Fatalf("Rejected() did not invoke the onreject callback")
	}
}
