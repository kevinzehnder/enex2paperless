package errs

import "net/http"

// custom error struct
type RestError struct {
	Status        int    `json:"status"`
	Message       string `json:"message"`
	Detail        string `json:"detail,omitempty"`
	InternalError string `json:"internalError,omitempty"`
}

func (e RestError) Error() string {
	// return e.InternalError
	return e.Message
}

// returns a new 400 error
func NewBadRequestError(message string) RestError {
	return RestError{
		Message: message,
		Status:  http.StatusBadRequest,
	}
}

// returns a new 404 error
func NewNotFoundError(message string) RestError {
	return RestError{
		Message: message,
		Status:  http.StatusNotFound,
	}
}

// returns a new 500 error
func NewInternalServerError(message string) RestError {
	return RestError{
		Message:       "Internal Server Error",
		InternalError: message,
		Status:        http.StatusInternalServerError,
	}
}
