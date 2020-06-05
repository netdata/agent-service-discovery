package tag

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_glob(t *testing.T) {
	tests := map[string]struct {
		patterns  []string
		value     string
		wantFalse bool
	}{
		"one param, matches": {
			patterns: []string{"*"},
			value:    "value",
		},
		"one param, not matches": {
			patterns:  []string{"Value"},
			value:     "value",
			wantFalse: true,
		},
		"several params, last one matches": {
			patterns: []string{"not", "matches", "*"},
			value:    "value",
		},
		"several params, no matches": {
			patterns:  []string{"not", "matches", "really"},
			value:     "value",
			wantFalse: true,
		},
	}

	for name, test := range tests {
		name := fmt.Sprintf("name: %s, patterns: '%v', value: '%s'", name, test.patterns, test.value)

		if test.wantFalse {
			assert.Falsef(t, glob(test.value, test.patterns[0], test.patterns[1:]...), name)
		} else {
			assert.Truef(t, glob(test.value, test.patterns[0], test.patterns[1:]...), name)
		}
	}
}

func Test_regexp(t *testing.T) {
	tests := map[string]struct {
		patterns  []string
		value     string
		wantFalse bool
	}{
		"one param, matches": {
			patterns: []string{"^value$"},
			value:    "value",
		},
		"one param, not matches": {
			patterns:  []string{"^Value$"},
			value:     "value",
			wantFalse: true,
		},
		"several params, last one matches": {
			patterns: []string{"not", "matches", "va[lue]{3}"},
			value:    "value",
		},
		"several params, no matches": {
			patterns:  []string{"not", "matches", "val[^l]ue"},
			value:     "value",
			wantFalse: true,
		},
	}

	for name, test := range tests {
		name := fmt.Sprintf("name: %s, patterns: '%v', value: '%s'", name, test.patterns, test.value)

		if test.wantFalse {
			assert.Falsef(t, regExp(test.value, test.patterns[0], test.patterns[1:]...), name)
		} else {
			assert.Truef(t, regExp(test.value, test.patterns[0], test.patterns[1:]...), name)
		}
	}
}

func Test_equal(t *testing.T) {
	tests := map[string]struct {
		patterns  []string
		value     string
		wantFalse bool
	}{
		"one param, matches": {
			patterns: []string{"value"},
			value:    "value",
		},
		"one param, not matches": {
			patterns:  []string{"Value"},
			value:     "value",
			wantFalse: true,
		},
		"several params, last one matches": {
			patterns: []string{"not", "matches", "value"},
			value:    "value",
		},
		"several params, no matches": {
			patterns:  []string{"not", "matches", "Value"},
			value:     "value",
			wantFalse: true,
		},
	}

	for name, test := range tests {
		name := fmt.Sprintf("name: %s, patterns: '%v', value: '%s'", name, test.patterns, test.value)

		if test.wantFalse {
			assert.Falsef(t, equal(test.value, test.patterns[0], test.patterns[1:]...), name)
		} else {
			assert.Truef(t, equal(test.value, test.patterns[0], test.patterns[1:]...), name)
		}
	}
}

func Test_hasKey(t *testing.T) {
	tests := map[string]struct {
		keys      []string
		value     interface{}
		wantFalse bool
	}{
		"one param, matches": {
			keys:  []string{"key"},
			value: map[string]int{"key": 0},
		},
		"one param, not matches": {
			keys:      []string{"Key"},
			value:     map[string]int{"key": 0},
			wantFalse: true,
		},
		"several params, last one matches": {
			keys:  []string{"not", "matches", "key"},
			value: map[string]int{"key": 0},
		},
		"several params, no matches": {
			keys:      []string{"not", "matches", "really"},
			value:     map[string]int{"key": 0},
			wantFalse: true,
		},
		"map[string]int": {
			keys:  []string{"key"},
			value: map[string]int{"key": 0},
		},
		"map[string]string": {
			keys:  []string{"key"},
			value: map[string]string{"key": "value"},
		},
		"map[string]struct": {
			keys:  []string{"key"},
			value: map[string]struct{}{"key": {}},
		},
		"*map[string]struct": {
			keys:  []string{"key"},
			value: &map[string]struct{}{"key": {}},
		},
		"int": {
			keys:      []string{"key"},
			value:     1,
			wantFalse: true,
		},
		"struct{}": {
			keys:      []string{"key"},
			value:     struct{}{},
			wantFalse: true,
		},
		"*struct{}": {
			keys:      []string{"key"},
			value:     &struct{ key int }{0},
			wantFalse: true,
		},
		"nil": {
			keys:      []string{"key"},
			wantFalse: true,
		},
	}

	for name, test := range tests {
		name := fmt.Sprintf("name: %s, keys: '%v', value: '%v'", name, test.keys, test.value)
		value := reflect.ValueOf(test.value)

		if test.wantFalse {
			assert.Falsef(t, hasKey(value, test.keys[0], test.keys[1:]...), name)
		} else {
			assert.Truef(t, hasKey(value, test.keys[0], test.keys[1:]...), name)
		}
	}
}
