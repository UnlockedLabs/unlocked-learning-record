package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/UnlockedLabs/unlocked-learning-record/internal/database"
)

// serviceError is an error with the status and message a handler answers
// with.
type serviceError struct {
	Status  int
	Message string
	Err     error
}

func (svcErr serviceError) Error() string {
	return fmt.Sprintf("service error: %s", svcErr.Err.Error())
}

func newServiceError(err error, status int, msg string) serviceError {
	svcErr := serviceError{Status: status, Message: msg, Err: err}
	if dbErr, ok := err.(database.DBError); ok {
		svcErr.Err = errors.Join(errors.New(dbErr.Message), dbErr.InternalErr)
	}
	return svcErr
}

func newBadRequestServiceError(err error, msg string) serviceError {
	return newServiceError(err, http.StatusBadRequest, msg)
}

func newResponseServiceError(err error) serviceError {
	return newServiceError(err, http.StatusInternalServerError, "error writing response")
}

func newJSONReqBodyServiceError(err error) serviceError {
	return newServiceError(err, http.StatusBadRequest, "error reading json request")
}

func newInvalidIdServiceError(err error, idName string) serviceError {
	return newServiceError(err, http.StatusBadRequest, fmt.Sprintf("invalid %s", idName))
}

// newDatabaseServiceError answers with a DBError's status and message, and
// with a 500 and the error's text for any other error.
func newDatabaseServiceError(err error) serviceError {
	if dbErr, ok := err.(database.DBError); ok {
		return serviceError{Status: dbErr.Status, Message: dbErr.Message, Err: dbErr.InternalErr}
	}
	return serviceError{Status: http.StatusInternalServerError, Message: err.Error(), Err: err}
}
