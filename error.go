package api2go

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cention-sany/jsonapi"
)

// HTTPError is used for errors
type HTTPError struct {
	err    error
	msg    string
	status int
	E      []*jsonapi.ErrorObject
}

// NewHTTPError creates a new error with message and status code.
// `err` will be logged (but never sent to a client), `msg` will be sent and `status` is the http status code.
// `err` can be nil.
func NewHTTPError(err error, msg string, status int) HTTPError {
	return HTTPError{err: err, msg: msg, status: status}
}

func NewOnlyHTTPError(httpCode int) HTTPError {
	return HTTPError{msg: http.StatusText(httpCode), status: httpCode}
}

func NewCustomError(err error, httpCode, appCode int) HTTPError {
	title := http.StatusText(httpCode)
	return HTTPError{
		msg:    title,
		status: httpCode,
		E: []*jsonapi.ErrorObject{&jsonapi.ErrorObject{
			Title:  title,
			Status: strconv.Itoa(httpCode),
			Code:   strconv.Itoa(appCode),
			Detail: err.Error(),
		}},
	}
}

// Error returns a nice string represenation including the status
func (e HTTPError) Error() string {
	msg := fmt.Sprintf("http error (%d) %s and %d more errors", e.status, e.msg,
		len(e.E))
	if e.err != nil {
		msg += ", " + e.err.Error()
	}

	return msg
}

// Write wraps Render() to provide compatibility with older gin versions
func (e HTTPError) Write(w http.ResponseWriter) error {
	return e.Render(w)
}

// Error returns a nice string represenation including the status
func (e HTTPError) Render(w http.ResponseWriter) error {
	if len(e.E) == 0 {
		e.E = []*jsonapi.ErrorObject{{
			Title:  e.msg,
			Status: strconv.Itoa(e.status),
		}}
	}
	e.WriteContentType(w)
	return jsonapi.MarshalErrors(w, e.E)
}

// WriteContentType sets the content type
func (e HTTPError) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", jsonapi.MediaType)
}
