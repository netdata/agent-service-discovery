package tag

import (
	"reflect"
	"regexp"
	"sync"

	"github.com/gobwas/glob"
)

var funcMap = map[string]interface{}{
	"glob":   globAny,
	"regexp": regExpAny,
	"equal":  equalAny,
	"hasKey": hasKeyAny,
}

func globAny(value, pattern string, rest ...string) bool {
	switch len(rest) {
	case 0:
		return globOnce(value, pattern)
	default:
		return globOnce(value, pattern) || globAny(value, rest[0], rest[1:]...)
	}
}

func regExpAny(value, pattern string, rest ...string) bool {
	switch len(rest) {
	case 0:
		return regExpOnce(value, pattern)
	default:
		return regExpOnce(value, pattern) || regExpAny(value, rest[0], rest[1:]...)
	}
}

func equalAny(value, pattern string, rest ...string) bool {
	switch len(rest) {
	case 0:
		return value == pattern
	default:
		return value == pattern || equalAny(value, rest[0], rest[1:]...)
	}
}

func hasKeyAny(value reflect.Value, key string, rest ...string) bool {
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

func globOnce(value, pattern string) bool {
	g, _ := globStore(pattern)
	return g != nil && g.Match(value)
}

func regExpOnce(value, pattern string) bool {
	r, _ := regexpStore(pattern)
	return r != nil && r.MatchString(value)
}

// TODO: cleanup?
var globStore = func() func(pattern string) (glob.Glob, error) {
	var l sync.RWMutex
	store := make(map[string]struct {
		g   glob.Glob
		err error
	})

	return func(pattern string) (glob.Glob, error) {
		if pattern == "" {
			return nil, nil
		}
		l.Lock()
		defer l.Unlock()
		r, ok := store[pattern]
		if !ok {
			r.g, r.err = glob.Compile(pattern, '/')
			store[pattern] = r
		}
		return r.g, r.err
	}
}()

// TODO: cleanup?
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
