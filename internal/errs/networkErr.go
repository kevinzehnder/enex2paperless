package errs

import "errors"

type NetworkError struct {
	Err         error
	InternalErr error
	Call        string
}

func (e NetworkError) Error() string {
	return e.Err.Error()
}

func (e NetworkError) InternalError() string {
	return e.InternalErr.Error()
}

func NewNetworkError(err error) NetworkError {
	return NetworkError{
		Err:         errors.New("NetworkError"),
		InternalErr: err,
	}
}
