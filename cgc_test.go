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
	"sync"
	"testing"
)

func TestNewBuffered(t *testing.T) {
	ex := NewBuffered(42)
	if cap(ex) != 42 {
		t.Fail()
	}
}

func TestSubmit(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ex := New()
	go func() {
		ex.RunOnce(context.Background())
		wg.Done()
	}()
	res, err := ex.Submit(context.Background(), func(ctx context.Context) (interface{}, error) {
		return 42, nil
	})
	if res != 42 || err != nil {
		t.Fail()
	}
	wg.Wait()
}

func TestSubmitNoWait(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	ex := New()
	go func() {
		ex.RunOnce(context.Background())
		wg.Done()
	}()
	err := ex.SubmitNoWait(context.Background(), func(ctx context.Context) (interface{}, error) {
		wg.Done()
		return nil, nil
	})
	if err != nil {
		t.Fail()
	}
	wg.Wait()
}

func TestClose(t *testing.T) {
	ex := New()
	close(ex)
	err := ex.RunLoop(context.Background())
	if err != nil {
		t.Fail()
	}
}

func TestCancel(t *testing.T) {
	ex := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ex.RunLoop(ctx)
	if err != context.Canceled {
		t.Fail()
	}
}

func TestSubmitCancel(t *testing.T) {
	ex := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := ex.Submit(ctx, func(ctx context.Context) (interface{}, error) {
		t.Fail()
		return nil, nil
	})
	if res != nil || err != context.Canceled {
		t.Fail()
	}
	err = ex.RunOnce(ctx)
	if err != context.Canceled {
		t.Fail()
	}
}

func TestSubmitNoWaitCancel(t *testing.T) {
	ex := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ex.SubmitNoWait(ctx, func(ctx context.Context) (interface{}, error) {
		t.Fail()
		return nil, nil
	})
	if err != context.Canceled {
		t.Fail()
	}
	err = ex.RunOnce(ctx)
	if err != context.Canceled {
		t.Fail()
	}
}

func TestJoinContext(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ex := New()
	go func() {
		ctx1, cancel1 := context.WithCancel(context.Background())
		err := ex.RunOnce(ctx1)
		if err != nil {
			t.Fail()
		}
		cancel1()
		wg.Done()
	}()
	ctx2, cancel2 := context.WithCancel(context.Background())
	res, err := ex.Submit(ctx2, func(ctx context.Context) (interface{}, error) {
		return 42, nil
	})
	if res != 42 || err != nil {
		t.Fail()
	}
	cancel2()
	wg.Wait()
}

func TestBrokenResultChan(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	ex := New()
	go func() {
		r := <-ex
		close(r.result)
		wg.Done()
	}()

	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != "cgc: result channel closed without any results" {
				t.Fail()
			}
		}()
		ex.Submit(context.Background(), func(ctx context.Context) (interface{}, error) {
			t.Fail()
			return nil, nil
		})
	}()
	wg.Wait()
}
