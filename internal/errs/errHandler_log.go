package errs

import (
	"errors"

	"github.com/rs/zerolog/log"
)

type LogErrorHandler struct {
}

func NewLogErrorHandler() *LogErrorHandler {
	return &LogErrorHandler{}
}

type ErrorHandler interface {
	Handle(error)
	HandleWithRetry(error) bool
}

func (l *LogErrorHandler) Handle(err error) {

	if err == nil {
		log.Info().Msg("No error to handle.")
		return
	}

	var restErr RestError
	var apiErr ApiCallError
	var networkErr NetworkError

	switch {
	case errors.As(err, &restErr):
		log.Error().
			Msgf("RestError: %s", err.Error())

	case errors.As(err, &apiErr):
		e := log.Error()
		e.Err(apiErr)
		if apiErr.InternalError != nil {
			e = e.Str("internalError", apiErr.InternalError.Error())
		}
		if apiErr.Call != "" {
			e = e.Str("call", apiErr.Call)
		}
		if apiErr.StatusCode != 0 {
			e.Int("statusCode", apiErr.StatusCode)
		}
		if apiErr.ResponseJson != nil {
			e = e.RawJSON("response", apiErr.ResponseJson)
		}
		e.Msg(err.Error())

	case errors.As(err, &networkErr):
		log.Error().
			Err(networkErr).
			Str("internalError", networkErr.InternalError()).
			Str("call", networkErr.Call).
			Msg(err.Error())

	// Handling unknown or generic errors
	default:
		log.Error().
			Err(err).
			Msg(err.Error())
	}
}

func (l *LogErrorHandler) HandleWithRetry(err error) bool {

	var restErr *RestError
	var retryableErr *RetryableErr

	switch {
	// if error is a RestErr
	case errors.As(err, &restErr):
		log.Debug().
			Msgf("RestError: %s", err.Error())
		return false

	case errors.As(err, &retryableErr):
		log.Debug().
			Msgf("RetryableError: %s", err.Error())
		return true

	default:
		log.Error().
			Msgf("InternalServerError: %v", err.Error())
		return false
	}
}
