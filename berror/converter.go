package berror

import (
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/lamber92/go-brick/berror/bcode"
	"github.com/lamber92/go-brick/berror/bstatus"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

var (
	defConv              Converter = newDefaultConverter()
	defCustomizedMapping           = sync.Map{}
)

// ReplaceConverter replace built-in Converter impl
func ReplaceConverter(c Converter) {
	defConv = c
}

// RegisterConvHook register hook function.
// nb. if you need this Hook, call it when you initialize the program.
func RegisterConvHook(hook HookFunc) {
	defConv.Hook(hook)
}

// RegisterCustomizedMapping register customized mapping for source err -> target err
func RegisterCustomizedMapping(src error, target error) error {
	if _, ok := defCustomizedMapping.Load(src); ok {
		return NewAlreadyExists(nil, "conflict occurs, source key already exists")
	}
	defCustomizedMapping.Store(src, target)
	return nil
}

// Convert according to the built-in rules, convert the incoming error to Error.
func Convert(err error, reason string, detail ...any) error {
	var d any
	if len(detail) > 0 {
		d = detail[0]
	}
	return defConv.Convert(err, reason, d)
}

// Convertf Convert with format
func Convertf(err error, format string, args ...any) error {
	return defConv.Convert(err, fmt.Sprintf(format, args...), nil)
}

// ConvertWithOption according to the built-in rules, convert the incoming error to Error.
func ConvertWithOption(err error, reason string, detail any, options ...ConvOption) error {
	return defConv.Convert(err, reason, detail, options...)
}

type defaultConverter struct {
	once sync.Once
	hook HookFunc
}

func newDefaultConverter() *defaultConverter {
	return &defaultConverter{}
}

// Convert when all error types are encountered, they are automatically wrapped as Error types.
// if you don't want to deal with the Error type, use the IgnoreWrapError option.
func (dc *defaultConverter) Convert(err error, reason string, detail any, options ...ConvOption) error {
	if err == nil {
		return NewWithSkip(err, bstatus.New(bcode.OK, reason, detail), 1)
	}
	// check it's berror.Error or not
	if orig, ok := err.(Error); ok {
		// check ignore warping.
		for _, v := range options {
			if v2, ok2 := v.(*defaultOption); ok2 && v2.Code == ignoreWrap {
				return err
			}
		}
		// keep the original Code and generate a new Error to wrap err.
		code := orig.Status().Code()
		return NewWithSkip(err, bstatus.New(code, reason, detail), 1)
	}
	if dc.hook != nil {
		return dc.hook(err, reason, detail, options...)
	}

	// TODO: Transfer the following business error codes to a custom error code mapping table
	if tmp, ok := defCustomizedMapping.Load(err); ok {
		return tmp.(error)
	}
	// check gorm/redis error
	switch err {
	case gorm.ErrRecordNotFound, redis.Nil:
		return NewWithSkip(err, bstatus.New(bcode.NotFound, reason, detail), 1)
	}
	// check it's grpc error or not
	if gerr, ok := status.FromError(err); ok && gerr != nil {
		code := bcode.FromGRPCCode(gerr.Code())
		return NewWithSkip(err, bstatus.New(code, reason, detail), 1)
	}
	// unknown error
	return NewWithSkip(err, bstatus.New(bcode.Unknown, reason, detail), 1)
}

func (dc *defaultConverter) Hook(f HookFunc) {
	dc.once.Do(func() {
		dc.hook = f
	})
}

const (
	ignoreWrap = 1
)

type defaultOption struct {
	Code int
}

// IgnoreWrapError if original error type is berror.Error,
// do not wrap the original error, return directly.
// only valid for berror.Error
func IgnoreWrapError() ConvOption {
	return &defaultOption{Code: ignoreWrap}
}
