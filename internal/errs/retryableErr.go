package errs

// custom error struct
type RetryableErr struct {
	Message       string `json:"message"`
	Detail        string `json:"detail,omitempty"`
	InternalError string `json:"internalError,omitempty"`
}

func (e RetryableErr) Error() string {
	return e.Message
}

// returns a new 400 error
func NewRetryableError(message string) *RetryableErr {
	return &RetryableErr{
		Message: message,
	}
}
