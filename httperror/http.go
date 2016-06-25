package httperror

import (
	"net/http"
	"net/http/httputil"
)

type ResponseError struct {
	Err      string
	Response *http.Response
}

func (e *ResponseError) Error() string {
	return e.Err
}

func (e *ResponseError) DumpRequest() string {
	b, err := httputil.DumpRequestOut(e.Response.Request, true)
	if err == nil {
		return string(b)
	}

	return ""
}

func NewResponseError(err string, resp *http.Response) *ResponseError {
	return &ResponseError{Err: err, Response: resp}
}
