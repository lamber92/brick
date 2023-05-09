package berror

import (
	"errors"
	"go-brick/berror/bcode"
	"go-brick/berror/bstatus"
	"runtime"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

const maxStackDepth = 32

var jsonStdIter = jsoniter.ConfigCompatibleWithStandardLibrary

// defaultError
// Provide built-in error status carrier
type defaultError struct {
	err    error          // original error
	status bstatus.Status // business information
	stack  []uintptr      // stack information when this object(*defaultError) was created
}

// New create and return an error containing a code and reason.
// If the parameter 'err' is passed in, it will wrap err.
func New(status bstatus.Status, err ...error) Error {
	e := &defaultError{
		stack:  callers(),
		status: status,
	}
	if len(err) > 0 {
		e.err = err[0]
	}
	return e
}

// Error output error information in string format
func (d *defaultError) Error() string {
	if d == nil {
		return ""
	}
	str, _ := jsonStdIter.MarshalToString(d.format())
	return str
}

// Status get main status
func (d *defaultError) Status() bstatus.Status {
	if d == nil {
		return bstatus.Unknown
	}
	return d.status
}

// TraceInfo Basic unit of position information when an error occurs
type TraceInfo struct {
	Func string // function name
	File string // file name
	Line int    // line
}

// Tracking list the error tracking information that has been collected
// TODO: Refer to this method to optimize: go.uber.org\zap@v1.24.0\stacktrace.go
func (d *defaultError) Tracking(depth ...int) []*TraceInfo {
	if d == nil || d.stack == nil || len(d.stack) == 0 {
		return []*TraceInfo{}
	}
	var (
		s   = make([]*TraceInfo, 0, len(d.stack))
		max = len(d.stack)
	)
	if depth != nil && len(depth) > 0 {
		max = depth[0]
	}

	for index, p := range d.stack {
		if fn := runtime.FuncForPC(p - 1); fn != nil {
			file, line := fn.FileLine(p - 1)
			// Avoid stack string like "`autogenerated`"
			if strings.Contains(file, "<") {
				continue
			}
			if index >= max {
				break
			}
			s = append(s, &TraceInfo{
				Func: fn.Name(),
				File: file,
				Line: line,
			})
		}
	}
	return s
}

// Wrap nest the specified error into error chain.
// Notice: will overwrite the original internal error
func (d *defaultError) Wrap(err error) error {
	if d == nil {
		return nil
	}
	d.err = err
	return d
}

// Unwrap returns the next error in the error chain.
func (d *defaultError) Unwrap() error {
	if d == nil {
		return nil
	}
	return d.err
}

// Is reports whether any error in error chain matches target.
func (d *defaultError) Is(target error) bool {
	return errors.Is(d, target)
}

// As finds the first error in error chain that matches target, and if one is found, sets
// target to that error value and returns true. Otherwise, it returns false.
func (d *defaultError) As(target any) bool {
	return errors.As(d, target)
}

type summary struct {
	Code   bcode.Code  `json:"code"`
	Reason string      `json:"reason"`
	Detail interface{} `json:"detail"`
	Sub    *summary    `json:"sub"`
}

func (d *defaultError) format() *summary {
	if d == nil || d.status == nil {
		return nil
	}
	return &summary{
		Code:   d.status.Code(),
		Reason: d.status.Reason(),
		Detail: d.status.Detail(),
		Sub:    d.format(),
	}
}

// newWithSkip
// create and return an error containing the stack trace.
// @offset: offset stack depth
func newWithSkip(err error, status bstatus.Status, offset int) Error {
	return &defaultError{
		err:    err,
		status: status,
		stack:  callers(offset),
	}
}

// callers
// Get stack information ptr
// TODO: Refer to this method to optimize: go.uber.org\zap@v1.24.0\stacktrace.go
func callers(skip ...int) []uintptr {
	var (
		pcs [maxStackDepth]uintptr
		n   = 3 // Because the call to this func has gone through 3 layers
	)
	if len(skip) > 0 {
		n += skip[0]
	}
	return pcs[:runtime.Callers(n, pcs[:])]
}
