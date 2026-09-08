package client

import "testing"

func TestAPIErrorErrorNeverEmpty(t *testing.T) {
	cases := []struct {
		err  APIError
		want string
	}{
		{APIError{Message: "nope", Code: "x", Status: 400}, "nope"},
		{APIError{Code: "unauthorized", Status: 401}, "unauthorized"},
		{APIError{Status: 401}, "http 401"},
		{APIError{}, "http 0"},
	}
	for _, tc := range cases {
		got := tc.err.Error()
		if got == "" {
			t.Errorf("%+v: Error() must not be empty", tc.err)
		}
		if got != tc.want {
			t.Errorf("%+v: got %q want %q", tc.err, got, tc.want)
		}
	}
}
