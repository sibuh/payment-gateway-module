package domain

import (
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/labstack/echo/v4"
)

type Error struct {
	Code        int                    `json:"code"`
	Message     string                 `json:"message"`
	Description string                 `json:"description"`
	Args        map[string]interface{} `json:"params"`
	Err         error                  `json:"err"`
	File        string                 `json:"-"`
	Line        int                    `json:"-"`
	Func        string                 `json:"-"`
	Stack       string                 `json:"-"`
}

func NewError(code int, message string, description string, err error, args map[string]interface{}) Error {
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	return Error{
		Code:        code,
		Message:     message,
		Description: description,
		Err:         err,
		Args:        args,
		File:        file,
		Line:        line,
		Func:        fn.Name(),
		Stack:       string(debug.Stack()),
	}
}

func (e Error) Error() string {
	return fmt.Sprintf("Message:%d: Description:%s Cause:%s", e.Code, e.Message, e.Err.Error())
}

func (e Error) ErrorCode() int {
	return e.Code
}

func (e Error) ErrorArgs() map[string]interface{} {
	return e.Args
}

func (e Error) Unwrap() error {
	return e.Err
}

type ErrorResponse struct {
	Code        int            `json:"code"`
	Message     string         `json:"message"`
	Description string         `json:"description,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

func ErrorHandler(err error, c echo.Context) {
	var (
		code        = http.StatusInternalServerError
		message     = "internal server error"
		description = ""
		params      map[string]interface{}
		internalErr error
		Line        = ""
		File        = ""
		Func        = ""
		Stack       = ""
	)

	switch e := err.(type) {

	// Your custom error
	case Error:
		code = e.Code
		message = e.Message
		description = e.Description
		params = e.Args
		internalErr = e.Err
		Line = fmt.Sprintf("%d", e.Line)
		File = e.File
		Func = e.Func
		Stack = e.Stack

	// Echo HTTP error
	case *echo.HTTPError:
		code = e.Code
		message = fmt.Sprint(e.Message)
		internalErr = e.Internal

	default:
		internalErr = err
	}

	//LOG INTERNAL ERROR
	if internalErr != nil {
		c.Logger().Errorf(
			"error=%+v code=%d message=%s params=%v file=%s line=%s func=%s stack=%s",
			internalErr,
			code,
			message,
			params,
			File,
			Line,
			Func,
			Stack,
		)
	} else {
		c.Logger().Errorf(
			"error=%+v code=%d message=%s params=%v file=%s line=%s func=%s stack=%s",
			err,
			code,
			message,
			params,
			File,
			Line,
			Func,
			Stack,
		)
	}

	// Response already sent?
	if c.Response().Committed {
		return
	}

	// Write safe response
	_ = c.JSON(code, ErrorResponse{
		Code:        code,
		Message:     message,
		Description: description,
		Params:      params,
	})
}
