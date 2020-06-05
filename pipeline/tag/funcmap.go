package tag

import (
	"path"
	"reflect"
	"regexp"
	"sync"
)

var funcMap = map[string]interface{}{
	"glob":   glob,
	"regexp": regExp,
	"equal":  equal,
	"hasKey": hasKey,
}

func glob(value, pattern string, rest ...string) bool {
	switch len(rest) {
	case 0:
		return _glob(value, pattern)
	default:
		return _glob(value, pattern) || glob(value, rest[0], rest[1:]...)
	}
}

func regExp(value, pattern string, rest ...string) bool {
	switch len(rest) {
	case 0:
		return _regExp(value, pattern)
	default:
		return _regExp(value, pattern) || regExp(value, rest[0], rest[1:]...)
	}
}

func equal(value, pattern string, rest ...string) bool {
	switch len(rest) {
	case 0:
		return value == pattern
	default:
		return value == pattern || equal(value, rest[0], rest[1:]...)
	}
}

func hasKey(value reflect.Value, key string, rest ...string) bool {
	value = reflect.Indirect(value)
	if value.Kind() != reflect.Map {
		return false
	}
	mr := value.MapRange()
	for mr.Next() {
		if mr.Key().String() == key {
			return true
		}
		for _, k := range rest {
			if mr.Key().String() == k {
				return true
			}
		}
	}
	return false
}

func _glob(value, pattern string) bool {
	ok, _ := path.Match(pattern, value)
	return ok
}

func _regExp(value, pattern string) bool {
	r, _ := regexpStore(pattern)
	return r != nil && r.MatchString(value)
}

var regexpStore = func() func(pattern string) (*regexp.Regexp, error) {
	var l sync.RWMutex
	store := make(map[string]struct {
		r   *regexp.Regexp
		err error
	})

	return func(pattern string) (*regexp.Regexp, error) {
		if pattern == "" {
			return nil, nil
		}
		l.Lock()
		defer l.Unlock()
		r, ok := store[pattern]
		if !ok {
			r.r, r.err = regexp.Compile(pattern)
			store[pattern] = r
		}
		return r.r, r.err
	}
}()
