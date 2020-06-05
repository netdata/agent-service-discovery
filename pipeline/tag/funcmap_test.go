package tag

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_glob(t *testing.T) {
	tests := []struct {
		pattern   string
		value     string
		wantFalse bool
	}{
		{pattern: "*", value: "value"},
		{pattern: "*alu*", value: "value"},
		{pattern: "?alu?", value: "value"},
		{pattern: "val", value: "value", wantFalse: true},
	}

	for i, test := range tests {
		name := fmt.Sprintf("pattern: '%s', value: '%s' (%d)", test.pattern, test.value, i+1)

		if test.wantFalse {
			assert.Falsef(t, glob(test.value, test.pattern), name)
		} else {
			assert.Truef(t, glob(test.value, test.pattern), name)
		}
	}
}

func Test_globAny(t *testing.T) {
	tests := []struct {
		pattern   string
		value     string
		wantFalse bool
	}{
		{pattern: "*", value: "value"},
		{pattern: "*alu*", value: "value"},
		{pattern: "?alu?", value: "value"},
		{pattern: "v *", value: "value"},
		{pattern: "v *alu*", value: "value"},
		{pattern: "v ?alu?", value: "value"},
		{pattern: "v val", value: "value", wantFalse: true},
		{pattern: "", value: "value", wantFalse: true},
	}

	for i, test := range tests {
		name := fmt.Sprintf("pattern: '%s', value: '%s' (%d)", test.pattern, test.value, i+1)

		if test.wantFalse {
			assert.Falsef(t, globAny(test.value, test.pattern), name)
		} else {
			assert.Truef(t, globAny(test.value, test.pattern), name)
		}
	}
}

func Test_regexp(t *testing.T) {
	tests := []struct {
		pattern   string
		value     string
		wantFalse bool
	}{
		{pattern: "^value$", value: "value"},
		{pattern: "alu", value: "value"},
		{pattern: "va[lue]{3}", value: "value"},
		{pattern: "", value: "value", wantFalse: true},
		{pattern: "val[^l]ue", value: "value", wantFalse: true},
	}

	for i, test := range tests {
		name := fmt.Sprintf("pattern: '%s', value: '%s' (%d)", test.pattern, test.value, i+1)

		if test.wantFalse {
			assert.Falsef(t, regExp(test.value, test.pattern), name)
		} else {
			assert.Truef(t, regExp(test.value, test.pattern), name)
		}
	}
}

func Test_eqAny(t *testing.T) {
	tests := []struct {
		pattern   string
		value     string
		wantFalse bool
	}{
		{pattern: "1 2 3 4 5 9001", value: "9001"},
		{pattern: "1 2 3 4 5   9001", value: "9001"},
		{pattern: "9001", value: "9001"},
		{pattern: "  9001  ", value: "9001"},
		{pattern: "1,9001", value: "9001", wantFalse: true},
		{pattern: "", value: "9001", wantFalse: true},
		{pattern: "900 1", value: "9001", wantFalse: true},
	}

	for i, test := range tests {
		name := fmt.Sprintf("pattern: '%s', value: '%s' (%d)", test.pattern, test.value, i+1)

		if test.wantFalse {
			assert.Falsef(t, eqAny(test.value, test.pattern), name)
		} else {
			assert.Truef(t, eqAny(test.value, test.pattern), name)
		}
	}
}

func Test_hasKey(t *testing.T) {
	tests := []struct {
		key       string
		value     interface{}
		wantFalse bool
	}{
		{key: "key", value: map[string]int{"key": 0}},
		{key: "key", value: map[string]string{"key": "value"}},
		{key: "key", value: map[string]struct{}{"key": {}}},
		{key: "key", value: &map[string]struct{}{"key": {}}},
		{key: "key", value: map[string]int{"value": 0}, wantFalse: true},
		{key: "", value: map[string]int{"key": 0}, wantFalse: true},
		{key: "key", value: 1, wantFalse: true},
		{key: "key", value: struct{}{}, wantFalse: true},
		{key: "key", value: &struct{ key int }{0}, wantFalse: true},
		{key: "key", wantFalse: true},
	}

	for i, test := range tests {
		name := fmt.Sprintf("key: '%s', value: '%#v' (%d)", test.key, test.value, i+1)
		value := reflect.ValueOf(test.value)

		if test.wantFalse {
			assert.Falsef(t, hasKey(value, test.key), name)
		} else {
			assert.Truef(t, hasKey(value, test.key), name)
		}
	}
}
