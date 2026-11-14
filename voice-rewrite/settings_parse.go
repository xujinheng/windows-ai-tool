package main

import "net/url"

func urlParse(s string) (string, error) {
	u, e := url.Parse(s)
	if e != nil {
		return "", e
	}
	return u.Port(), nil
}
