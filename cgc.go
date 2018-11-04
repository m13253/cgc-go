/*
   cgc -- Cross Goroutine Calls
   Copyright (C) 2018 Star Brilliant <coder@poorlab.com>

   Permission is hereby granted, free of charge, to any person obtaining a
   copy of this software and associated documentation files (the "Software"),
   to deal in the Software without restriction, including without limitation
   the rights to use, copy, modify, merge, publish, distribute, sublicense,
   and/or sell copies of the Software, and to permit persons to whom the
   Software is furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in
   all copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
   FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
   DEALINGS IN THE SOFTWARE.
*/

// Package cgc is a package that allows you pass a function for another
// goroutine to execute it.
//
// It is useful when some logic must be run on a separate thread and you want to
// wait for the result, or you want to cancel a foreign function with Go's
// context cancellation mechanism.
//
// The package provides a event queue named Executor. You can make use of
// Executor to listen to requests from one goroutine, and send requests to it
// from any other goroutines. Please refer to documentations of each function
// for details.
package cgc

import (
	"context"
	"io"

	"github.com/LK4D4/joincontext"
)

type (
	// Func is a function to submit to another goroutine to execute.
	Func func(ctx context.Context) (interface{}, error)

	result struct {
		val interface{}
		err error
	}

	// Request is a message body passed from the caller to the callee.
	Request struct {
		Func    Func
		Context context.Context
		result  chan<- *result
	}

	// Executor is a cross-goroutine execution unit.
	Executor chan *Request
)

// New creates a new unbuffered Executor.
func New() Executor {
	return make(Executor)
}

// NewBuffered creates a new Executor with specified buffer length.
func NewBuffered(bufferLength uint) Executor {
	return make(Executor, bufferLength)
}

// RunLoop keeps executing requests from the executor until ctx is canceled.
//
// This function should be called from the callee goroutine, either ctx or
// r.Context may cancel the inner function.
//
// Instead of using RunLoop, you can also use
//     loop:
//         for {
//             select {
//                 case req := <-executor:
//                     cgc.RunOneRequest(ctx, req)
//                 case <-ctx.Done():
//                     break loop
//                 /* case other events: */
//             }
//         }
// manually if the callee goroutine has more things to do than just listening to
// requests.
func (e Executor) RunLoop(ctx context.Context) error {
	for {
		err := e.RunOnce(ctx)
		if err == io.EOF {
			return nil
		}
		if err == context.Canceled {
			return err
		}
	}
}

// RunOnce executes one request from the executor.
//
// This function should be called from the callee goroutine, either ctx or
// r.Context may cancel the inner function.
//
// Instead of using RunOnce, you can also use
//     req := <-executor
//     cgc.RunOneRequest(ctx, req)
func (e Executor) RunOnce(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	case r, ok := <-e:
		if !ok {
			return io.EOF
		}
		RunOneRequest(ctx, r)
		return nil
	}
}

// Submit submits a request to the executor and wait for the result.
//
// This function should be called from the caller goroutine, either ctx or the
// context at the callee goroutine may cancel the request.
func (e Executor) Submit(ctx context.Context, f Func) (interface{}, error) {
	resultChan := make(chan *result, 1)
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	case e <- &Request{
		Func:    f,
		Context: ctx,
		result:  resultChan,
	}:
	}
	res, ok := <-resultChan
	if !ok {
		panic("cgc: result channel closed without any results")
	}
	return res.val, res.err
}

// SubmitNoWait submits a request, wait for the request to be received, but does
// not wait for the result.
//
// This function should be called from the caller goroutine, either ctx or the
// context at the callee goroutine may cancel the request.
func (e Executor) SubmitNoWait(ctx context.Context, f Func) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	case e <- &Request{
		Func:    f,
		Context: ctx,
		result:  nil,
	}:
	}
	return nil
}

// RunOneRequest executes on request that is already extracted from an executor.
//
// To extract a request from an executor, use
//     req := <-executor
//
// This function should be called from the callee goroutine, either ctx or
// r.Context may cancel the inner function.
//
// Instead of using RunOneRequest, you can also use
//     executor.RunOnce(ctx)
func RunOneRequest(ctx context.Context, r *Request) {
	joinedCtx, joinedCancel := ctx, context.CancelFunc(nil)
	if ctx != r.Context {
		joinedCtx, joinedCancel = joincontext.Join(ctx, r.Context)
	}
	val, err := r.Func(joinedCtx)
	if joinedCancel != nil {
		joinedCancel()
	}
	if r.result != nil {
		r.result <- &result{
			val: val,
			err: err,
		}
		close(r.result)
	}
}
