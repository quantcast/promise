package promise

import (
	"errors"
	"testing"
)

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
