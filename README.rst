===============================================================================
An implementation of Promises (akin to Promises/A+) in Golang.
===============================================================================

This package provides a concurrently safe composable placeholder type for the
Go programming language.

Rationale
===============================================================================
While concurrency in Go programs are generally managed using channels, and this
is a good model for a great many workflows, it is difficult at times to manage
concurrent workflows which are constructed dynamically using this model.
Channels need to have known readers, writers, and passing them around and
composing them is difficult.

Promises, meanwhile, provide a very natural way to define a parallel program as
a series of composable operations and merge points. This implementation of
promises enables programs written in Go to use this model when it is a better
fit.

Synopsis
===============================================================================

::

    p := promise.Promise()

    squared := p.Then(func(value interface{}) interface{} {
            val, _ := value.(int)

            return val * val
    })

    cubed := squared.Then(func(value interface{}) interface{} {
            val, _ := value.(int)

            return val * val * val
    })

    // The above code will now execute.
    promise.Complete(2)

Combining Promises
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Promises provide a way to defer computation until a future state transition,
but to compose those computations eagerly into a chain of operations which will
run at a later point in time. In this API, there are two basic types of
compositions. ``Then`` compositions modify a value when it becomes available,
and produce a promise for this modified value.  ``Combine`` compositions
can construct a new promise after observing the value associated with the
promise, and produce another promise which is completed when the returned
promise is completed.

::

    // This returns a new ``CompletedPromise``, but it could also return a
    // ``CompletablePromise`` instead.
    combined := p.Combine(func(value interface{}) Thenable {
            val, _ := value.(int)

            return Completed(val + 3)
    })

Creating Promises
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
There are three types of promises, each of which implements the ``Thenable``
interface.

================== ================ =========================================
Type               Constructor      Description
------------------ ---------------- -----------------------------------------
CompletablePromise ``Promise()``    The composable placeholder for a value
                                    which will be made available at a later
                                    time. This is the primary type of Promise
                                    used by most programs.
CompletedPromise   ``Completed(v)`` A promise whose value is already available.
                                    The value ``v`` is provided to the
                                    constructor.
RejectedPromise    ``Rejected(c)``  A promise which is in a failed state, the
                                    cause ``c`` is provided to the constructor
                                    and must be of type ``error``.
================== ================ =========================================

Completing Promises
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Both Rejected and Completed promises are already in a *completed* state when
they are initialized.  Completable promises, conversely, are initialized in an
incomplete state, and they therefore encapsulate one of two possible state
transitions.

The most obvious and natural state transition is that some deferred or
asynchronous computation produces a value, completing the promise. This is
achieved by invoking the ``Complete`` method. Completion transitions the
promise from a state of being *incomplete* to being *completed*, and execute
any composed computations which may depend on the value of this promise. The
execution coincidentally happens from within the goroutine which produced the
value. All promises which have been created by composing a computation with the
promise being completed will thereafter be completed as well, executing their
computations and completing any promises which may have been composed from
those executions, et cetera.

The less obvious but nevertheless rather important alternate state transition
is to reject a promise. This is a means for handling errors within chains of
promises. Once the ``Reject`` method has been called on a
``CompletablePromise``, any composed promises which depend on that promise are
also rejected.

In both cases, once the promise transitions to a completed state, it can not
transition again and any attempt to do so is a fatal error. Further, it
afterward becomes a ``CompletedPromise`` or ``RejectedPromise``, respectively,
and any invocation of either the ``Then``, ``Combine`` or ``Catch`` methods
produce promises of those types. This is largely unimportant for the user of
the API, but it does have the implication that the computations are at that
point actually executed by the goroutine which invokes the ``Then``,
``Combine`` or ``Catch`` methods, respectively.

For most well written programs using promises, where the composed computations
actually run is completely inconsequential.

Using promises
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
The ``Combine`` and ``Then`` operations can be used to compute values or
compose computations of values, and that's great — but what happens when the
process is done? And what happens when the result of the computation needs to
be integrated with a tool-chain which doesn't use promises?

Every implementation of ``Theanble`` has a ``Get()`` method, which returns a
the result of the computation. It's signature is ``Get() (interface{}, error)``,
and it returns either the value of the computation, or optionally, an error. If
the promise is a ``CompletablePromise`` and it is in an *incomplete* state, the
method blocks until the promise is either ``Completed`` or ``Rejected``.

License
===============================================================================
This software is Copyright © 2016 Quantcast Corporation, and is provided under
the MIT license. See the ``LICENSE`` file for details.
