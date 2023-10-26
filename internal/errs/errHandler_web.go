package errs

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"
)

type WebErrorHandler struct {
}

func NewWebErrorHandler() *WebErrorHandler {
	return &WebErrorHandler{}
}

func (l *WebErrorHandler) Handle(err error, w http.ResponseWriter) {

	restErr := &RestError{}

	switch {
	// if error is a RestErr
	case errors.As(err, &restErr):
		// log error
		log.Debug().
			Str("InternalError", err.Error()).
			Msg("RestError")

		// add this for debugging purpose
		restErr.InternalError = err.Error()

		// http response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(restErr.Status)
		json.NewEncoder(w).Encode(restErr)

	default:
		// log error
		log.Error().
			Str("InternalError", err.Error()).
			Msg("InternalServerError")

		// http response
		p := NewInternalServerError(err.Error())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(p.Status)
		json.NewEncoder(w).Encode(p)
	}
}
