package model

import (
	"fmt"
	"sort"
	"strings"
)

type Tags map[string]struct{}

func NewTags() Tags {
	return Tags{}
}

func (t Tags) Merge(tags Tags) {
	for tag := range tags {
		if strings.HasPrefix(tag, "-") {
			delete(t, tag[1:])
		} else {
			t[tag] = struct{}{}
		}
	}
}

func (t Tags) String() string {
	ts := make([]string, 0, len(t))
	for key := range t {
		ts = append(ts, key)
	}
	sort.Strings(ts)
	return fmt.Sprintf("{%s}", strings.Join(ts, ", "))
}

func ParseTags(line string) (Tags, error) {
	words := strings.Fields(line)
	if len(words) == 0 {
		return NewTags(), nil
	}

	tags := NewTags()
	for _, tag := range words {
		if !isTagWordValid(tag) {
			return nil, fmt.Errorf("tags '%s' contains tag '%s' with forbidden symbol", line, tag)
		}
		tags[tag] = struct{}{}
	}
	return tags, nil
}

func MustParseTags(line string) Tags {
	tags, err := ParseTags(line)
	if err != nil {
		panic(fmt.Sprintf("tags '%s' parse error: %v", line, err))
	}
	return tags
}

func isTagWordValid(word string) bool {
	// valid:
	// ^[a-zA-Z][a-zA-Z0-9=_.]*$
	if strings.HasPrefix(word, "-") {
		word = word[1:]
	}
	if len(word) == 0 {
		return false
	}
	for i, b := range word {
		switch {
		default:
			return false
		case b >= 'a' && b <= 'z':
		case b >= 'A' && b <= 'Z':
		case b >= '0' && b <= '9' && i > 0:
		case (b == '=' || b == '_' || b == '.') && i > 0:
		}
	}
	return true
}
