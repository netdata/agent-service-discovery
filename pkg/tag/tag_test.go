package tag

import (
	"fmt"
	"testing"

	"github.com/netdata/sd/pkg/model"
)

func TestNew(t *testing.T) {
	tests := map[string]tagSimTest{
		"valid config": {
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
					},
				},
			},
		},
		"empty config": {
			cfg:     Config{},
			invalid: true,
		},
		"config rule->selector not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
					},
				},
			},
		},
		"config rule->selector bad syntax": {
			invalid: true,
			cfg: Config{
				{
					Selector: "!",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
					},
				},
			},
		},
		"config rule->tags not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
					},
				},
			},
		},
		"config rule->match not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
				},
			},
		},
		"config rule->match->selector bad syntax": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Selector: "!", Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
					},
				},
			},
		},
		"config rule->match->tags not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "", Cond: `{{eq .Class "wizard"}}`},
					},
				},
			},
		},
		"config rule->match->cond not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: ""},
					},
				},
			},
		},
		"config rule->match->cond unknown func": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: `{{error .Class "wizard"}}`},
					},
				},
			},
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

func TestManager_Tag(t *testing.T) {
	tests := map[string]tagSimTest{
		"3 rule service": {
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
						{Tags: "knight", Cond: `{{eq .Class "knight"}}`},
						{Tags: "cleric", Cond: `{{eq .Class "cleric"}}`},
					},
				},
				{
					Selector: "!unknown",
					Tags:     "candidate",
					Match: []MatchConfig{
						{Tags: "human", Cond: `{{eq .Race "human"}}`},
						{Tags: "elf", Cond: `{{eq .Race "elf"}}`},
						{Tags: "dwarf", Cond: `{{eq .Race "dwarf"}}`},
					},
				},
				{
					Selector: "candidate",
					Tags:     "-candidate",
					Match: []MatchConfig{
						{Tags: "teamup", Cond: `{{gt .Level 9000}}`},
					},
				},
			},
			values: []tagSimTestValue{
				{
					desc:         "all rules fail",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "fighter", Race: "orc", Level: 9001},
					expectedTags: model.Tags{"unknown": {}},
				},
				{
					desc:         "1st rule match",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "knight", Race: "undead", Level: 9001},
					expectedTags: model.Tags{"knight": {}},
				},
				{
					desc:         "1st, 2nd rules match",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "knight", Race: "human", Level: 8999},
					expectedTags: model.Tags{"knight": {}, "human": {}, "candidate": {}},
				},
				{
					desc:         "all rules match",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "wizard", Race: "human", Level: 9001},
					expectedTags: model.Tags{"wizard": {}, "human": {}, "teamup": {}},
				},
				{
					desc:         "all rules match",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "knight", Race: "dwarf", Level: 9001},
					expectedTags: model.Tags{"knight": {}, "dwarf": {}, "teamup": {}},
				},
				{
					desc:         "all rules match",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "cleric", Race: "elf", Level: 9001},
					expectedTags: model.Tags{"cleric": {}, "elf": {}, "teamup": {}},
				},
			},
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

func TestRule_Tag(t *testing.T) {
	tests := map[string]tagSimTest{
		"simple rule": {
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Match: []MatchConfig{
						{Selector: "human", Tags: "wizard", Cond: `{{eq .Class "wizard"}}`},
						{Tags: "missingkey", Cond: `{{eq .Name "yoda"}}`},
					},
				},
			},
			values: []tagSimTestValue{
				{
					desc:         "not match rule selector",
					target:       mockTarget{Class: "fighter"},
					expectedTags: nil,
				},
				{
					desc:         "not match rule match selector",
					target:       mockTarget{tags: model.Tags{"unknown": {}}, Class: "fighter"},
					expectedTags: model.Tags{"unknown": {}},
				},
				{
					desc:         "not match rule match condition",
					target:       mockTarget{tags: model.Tags{"unknown": {}, "human": {}}, Class: "fighter"},
					expectedTags: model.Tags{"unknown": {}, "human": {}},
				},
				{
					desc:         "match condition",
					target:       mockTarget{tags: model.Tags{"unknown": {}, "human": {}}, Class: "wizard"},
					expectedTags: model.Tags{"wizard": {}, "human": {}},
				},
				{
					desc:         "match condition missingkey error",
					target:       mockTarget{tags: model.Tags{"unknown": {}, "missingkey": {}}, Class: "knight"},
					expectedTags: model.Tags{"unknown": {}, "missingkey": {}},
				},
			},
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

func TestRule_Tag_UseCustomFunction(t *testing.T) {
	newSim := func(cond string) tagSimTest {
		return tagSimTest{
			cfg: Config{
				{
					Selector: "*",
					Tags:     "-nothing",
					Match: []MatchConfig{
						{Tags: "wizard", Cond: cond},
					},
				},
			},
			values: []tagSimTestValue{
				{
					target:       mockTarget{Class: "wizard", tags: model.Tags{"key": {}}},
					expectedTags: model.Tags{"key": {}, "wizard": {}},
				},
			},
		}
	}

	tests := map[string]tagSimTest{
		"glob":   newSim(`{{glob .Class "w*z*rd"}}`),
		"regexp": newSim(`{{regexp .Class "^w[iI]z.*d$"}}`),
		"eqAny":  newSim(`{{eqAny .Class "ranger knight cleric wizard"}}`),
		"hasKey": newSim(`{{hasKey .Tags "key"}}`),
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

type mockTarget struct {
	tags  model.Tags
	Class string
	Race  string
	Level int
}

func (m mockTarget) Tags() model.Tags { return m.tags }
func (mockTarget) Hash() uint64       { return 0 }
func (mockTarget) TUID() string       { return "" }

func (m mockTarget) String() string {
	return fmt.Sprintf("Class: %s, Race: %s, Level: %d, Tags: %s", m.Class, m.Race, m.Level, m.Tags())
}
