package client

import (
	"errors"
	"fmt"
	"strings"
)

type APIError struct {
	Status     int
	Code       string
	Message    string
	Candidates []AlbumMemberCandidate
}

func (e *APIError) Error() string {
	if e == nil {
		return "http 0"
	}
	var msg string
	switch {
	case e.Message != "":
		msg = e.Message
	case e.Code != "":
		msg = e.Code
	default:
		msg = fmt.Sprintf("http %d", e.Status)
	}
	if len(e.Candidates) == 0 {
		return msg
	}
	parts := make([]string, len(e.Candidates))
	for i, c := range e.Candidates {
		parts[i] = fmt.Sprintf("%s (%d)", c.Username, c.UserID)
	}
	return msg + ": " + strings.Join(parts, ", ")
}

func IsUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == 401
}
