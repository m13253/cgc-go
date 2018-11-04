# cgc-go: Cross Goroutine Calls library

[![](https://godoc.org/github.com/m13253/cgc-go?status.svg)](http://godoc.org/github.com/m13253/cgc-go)

Package cgc is a package that allows you pass a function for another goroutine
to execute it.

It is useful when some logic must be run on a separate thread and you want to
wait for the result, or you want to cancel a foreign function with Go's context
cancellation mechanism.

The package provides a event queue named `Executor`. You can make use of
`Executor` to listen to requests from one goroutine, and send requests to it
from any other goroutines.
