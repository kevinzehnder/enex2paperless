package errs

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-resty/resty/v2"
)

type ApiCallError struct {
	Err           error
	StatusCode    int
	ResponseJson  []byte
	Call          string
	InternalError error
}

func (e ApiCallError) Error() string {
	return e.Err.Error()
}

func NewAPICallError(info any) ApiCallError {
	err := ApiCallError{Err: errors.New("APICallError")}

	switch v := info.(type) {
	case *resty.Response:
		err.StatusCode = v.StatusCode()
		err.ResponseJson = validateJSONResponseBody(v)
	case string:
		err.InternalError = errors.New(v)
	case error:
		if v != nil {
			err.InternalError = v
		} else {
			err.InternalError = errors.New("Received nil error")
		}	
	default:
		err.InternalError = errors.New("Unknown type")
	}

	return err
}

// validate response body
func validateJSONResponseBody(res *resty.Response) []byte {
	contentType := res.Header().Get("Content-Type")

	// Skip validation for non-JSON content types, like files
	if strings.Contains(contentType, "image/jpeg") {
		return []byte(`{"non-json-content: true}`)
	}

	// validate JSON content
	var responseBody []byte
	var err error

	if json.Valid(res.Body()) {
		responseBody = res.Body()
	} else {
		responseBody, err = json.Marshal(res.String())
		if err != nil {
			responseBody = []byte("")
		}
	}

	return responseBody

}
