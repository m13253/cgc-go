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

package cgc

import (
	"context"
	"io"

	"github.com/LK4D4/joincontext"
)

type (
	// Func is a function to submit to another goroutine to execute
	Func func(ctx context.Context) (interface{}, error)

	result struct {
		val interface{}
		err error
	}

	// Request is a message body passed from the caller to the callee
	Request struct {
		Func    Func
		Context context.Context
		result  chan<- *result
	}

	// Executor is a cross-goroutine execution unit
	Executor chan *Request
)

// New creates a new unbuffered Executor
func New() Executor {
	return make(Executor)
}

// NewBuffered creates a new Executor with specified buffer length
func NewBuffered(bufferLength uint) Executor {
	return make(Executor, bufferLength)
}

// RunLoop keeps executing requests from the executor until ctx is canceled
//
// This function should be called from the callee goroutine
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

// RunOnce executes one request from the executor
//
// This function should be called from the callee goroutine
func (e Executor) RunOnce(ctx context.Context) error {
	select {
	case r, ok := <-e:
		if !ok {
			return io.EOF
		}
		return RunOneRequest(ctx, r)
	case <-ctx.Done():
		return context.Canceled
	}
}

// Submit submits a request to the executor and wait for the result
//
// This function should be called from the caller goroutine
func (e Executor) Submit(ctx context.Context, f Func) (interface{}, error) {
	resultChan := make(chan *result, 1)
	select {
	case e <- &Request{
		Func:    f,
		Context: ctx,
		result:  resultChan,
	}:
	case <-ctx.Done():
		return nil, context.Canceled
	}
	select {
	case r, ok := <-resultChan:
		if !ok {
			return nil, context.Canceled
		}
		return r.val, r.err
	case <-ctx.Done():
		return nil, context.Canceled
	}
}

// SubmitNoWait submits a request, wait for the request to be received, but does not wait for the result
//
// This function should be called from the caller goroutine
func (e Executor) SubmitNoWait(ctx context.Context, f Func) error {
	select {
	case e <- &Request{
		Func:    f,
		Context: ctx,
		result:  nil,
	}:
	case <-ctx.Done():
		return context.Canceled
	}
	return nil
}

// RunOneRequest executes on request that is already extracted from an executor
//
// Either ctx or r.Context may cancel the inner function
func RunOneRequest(ctx context.Context, r *Request) error {
	joinedCtx, joinedCancel := ctx, context.CancelFunc(nil)
	if ctx != r.Context {
		joinedCtx, joinedCancel = joincontext.Join(ctx, r.Context)
	}
	val, err := r.Func(joinedCtx)
	if joinedCancel != nil {
		joinedCancel()
	}
	if r.result == nil {
		return nil
	}
	defer close(r.result)
	select {
	case r.result <- &result{
		val: val,
		err: err,
	}:
	case <-ctx.Done():
		return context.Canceled
	}
	return nil
}
