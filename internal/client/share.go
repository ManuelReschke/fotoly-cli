package client

import (
	"errors"
	"net/url"
)

func ResolveShareURL(baseURL string, candidates ...string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		u, err := url.Parse(candidate)
		if err != nil {
			return "", err
		}
		if !u.IsAbs() {
			u = base.ResolveReference(u)
		}
		return u.String(), nil
	}
	return "", errors.New("no share URL")
}
