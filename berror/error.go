package berror

import (
	"errors"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"github.com/lamber92/go-brick/berror/bcode"
	"github.com/lamber92/go-brick/berror/bstatus"
	"github.com/lamber92/go-brick/bstack"
	"go.uber.org/zap/zapcore"
)

var jsonStdIter = jsoniter.ConfigCompatibleWithStandardLibrary

// defaultError
// Provide built-in error status carrier
type defaultError struct {
	err    error            // original error
	status bstatus.Status   // business information
	stack  bstack.StackList // stack information when this object(*defaultError) was created
}

// New create and return an error containing a code and reason.
// if the parameter 'err' is passed in, it will wrap err.
//
// nb 1. nesting call this function may create inaccurate stack cheapness,
// if necessary, use NewWithSkip instead.
//
// nb 2. if @err type is *defaultError,
// the @err stack will be inherited.
func New(status bstatus.Status, err ...error) Error {
	e := &defaultError{status: status}
	// check original err and try to inherit err-stack
	if len(err) > 0 {
		e.err = err[0]
		if orig, ok := e.err.(*defaultError); ok {
			e.stack = orig.stack
		}
	}
	// generate new stack info
	if e.stack == nil {
		e.stack = bstack.TakeStack(1, bstack.StacktraceMax)
	}
	return e
}

// NewWithSkip create and return an error containing the stack trace.
// @offset: offset stack depth
//
// nb. if @err type is *defaultError,
// the @err stack will be inherited.
func NewWithSkip(err error, status bstatus.Status, skip int) Error {
	e := &defaultError{
		err:    err,
		status: status,
	}
	// check original err and try to inherit err-stack
	if err != nil {
		e.err = err
		if orig, ok := e.err.(*defaultError); ok {
			e.stack = orig.stack
		}
	}
	// generate new stack info
	if e.stack == nil {
		e.stack = bstack.TakeStack(skip+1, bstack.StacktraceMax)
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

// Stack list the error tracking information that has been collected
func (d *defaultError) Stack() bstack.StackList {
	if d == nil || d.stack == nil || len(d.stack) == 0 {
		return bstack.StackList{}
	}
	return d.stack
}

// Cause returns the underlying cause of the error, if possible.
func (d *defaultError) Cause() error {
	if d == nil {
		return nil
	}
	return d.err
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (d *defaultError) Unwrap() error {
	if d == nil {
		return nil
	}
	return d.err
}

type summary struct {
	Code   bcode.Code `json:"code"`
	Reason string     `json:"reason"`
	Detail any        `json:"detail"`
	Next   any        `json:"next"`
}

func (d *defaultError) format() *summary {
	if d == nil || d.status == nil {
		return nil
	}
	sum := &summary{
		Code:   d.status.Code(),
		Reason: d.status.Reason(),
		Detail: d.status.Detail(),
	}
	if d.err == nil {
		sum.Next = nil
	} else {
		switch next := d.err.(type) {
		case *defaultError:
			sum.Next = next.format()
		default:
			sum.Next = next.Error()
		}
	}
	return sum
}

// MarshalLogObject zapcore.ObjectMarshaler impl
func (d *defaultError) MarshalLogObject(enc zapcore.ObjectEncoder) (err error) {
	// code/reason
	status := d.status
	enc.AddInt("code", status.Code().ToInt())
	enc.AddString("reason", status.Reason())
	// detail
	if status.Detail() != nil {
		if obj, ok := status.Detail().(zapcore.ObjectMarshaler); ok {
			_ = enc.AddObject("detail", obj)
		} else {
			_ = enc.AddReflected("detail", status.Detail())
		}
	}
	// nest error
	if d.err == nil {
		return
	}
	if next, ok := d.err.(*defaultError); ok {
		_ = enc.AddObject("next", next)
		return
	}
	enc.AddString("next", d.err.Error())
	return
}

// NewInvalidArgument create a invalid argument error
func NewInvalidArgument(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.InvalidArgument, reason, ds), 1)
}

// NewInvalidArgumentf create a invalid argument error with format
func NewInvalidArgumentf(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.InvalidArgument, fmt.Sprintf(format, args...), nil), 1)
}

// NewNotFound create a not found error
func NewNotFound(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.NotFound, reason, ds), 1)
}

// NewNotFoundf create a not found error with format
func NewNotFoundf(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.NotFound, fmt.Sprintf(format, args...), nil), 1)
}

// NewRequestTimeout create a request timeout error
func NewRequestTimeout(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.RequestTimeout, reason, ds), 1)
}

// NewRequestTimeoutf create a request timeout error with format
func NewRequestTimeoutf(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.RequestTimeout, fmt.Sprintf(format, args...), nil), 1)
}

// NewGatewayTimeout create a gateway timeout error
func NewGatewayTimeout(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.GatewayTimeout, reason, ds), 1)
}

// NewGatewayTimeoutf create a gateway timeout error with format
func NewGatewayTimeoutf(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.GatewayTimeout, fmt.Sprintf(format, args...), nil), 1)
}

// NewClientClose create a client close error
func NewClientClose(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.ClientClosed, reason, ds), 1)
}

// NewClientClosef create a client close error with format
func NewClientClosef(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.ClientClosed, fmt.Sprintf(format, args...), nil), 1)
}

// NewAlreadyExists create a already exists error
func NewAlreadyExists(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.AlreadyExists, reason, ds), 1)
}

// NewAlreadyExistsf create a already exists error with format
func NewAlreadyExistsf(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.AlreadyExists, fmt.Sprintf(format, args...), nil), 1)
}

// NewInternalError create a internal error
func NewInternalError(err error, reason string, detail ...any) error {
	var ds any = nil
	if len(detail) > 0 {
		ds = detail[0]
	}
	return NewWithSkip(err, bstatus.New(bcode.InternalError, reason, ds), 1)
}

// NewInternalErrorf create a internal error with format
func NewInternalErrorf(err error, format string, args ...any) error {
	return NewWithSkip(err, bstatus.New(bcode.InternalError, fmt.Sprintf(format, args...), nil), 1)
}

// IsCode determine whether the error code of err meets expectations.
func IsCode(err error, code bcode.Code) bool {
	if err == nil {
		return false
	}
	var e Error
	if ok := errors.As(err, &e); !ok {
		return false
	}
	if e.Status().Code().ToInt() == code.ToInt() {
		return true
	}
	return false
}
