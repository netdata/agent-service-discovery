package tag

import (
	"path"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

var condTmplFuncMap = map[string]interface{}{
	"glob":   glob,
	"regexp": regExp,
	"eqAny":  eqAny,
	"hasKey": hasKey,
}

func glob(value, pattern string) bool {
	ok, _ := path.Match(pattern, value)
	return ok
}

func regExp(value, pattern string) bool {
	r, _ := regexpStore(pattern)
	return r != nil && r.MatchString(value)
}

func eqAny(value, pattern string) bool {
	if idx := strings.IndexByte(pattern, ' '); idx != -1 {
		return value == pattern[:idx] || eqAny(value, pattern[idx+1:])
	}
	return value == pattern
}

func hasKey(value reflect.Value, key string) bool {
	value = reflect.Indirect(value)
	if value.Kind() != reflect.Map {
		return false
	}
	mr := value.MapRange()
	for mr.Next() {
		if mr.Key().String() == key {
			return true
		}
	}
	return false
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
