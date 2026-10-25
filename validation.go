package cacheinstance

import (
	"reflect"
	"strings"
)

func clean(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrInvalidArgument
	}
	return s, nil
}

func validInstance(v any) bool {
	return v != nil && reflect.ValueOf(v).Kind() == reflect.Pointer && !reflect.ValueOf(v).IsNil()
}
