package client

import "errors"

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == 401
}
