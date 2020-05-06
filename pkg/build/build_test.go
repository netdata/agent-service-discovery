package build

import (
	"fmt"
	"testing"

	"github.com/netdata/sd/pkg/model"
)

func TestNew(t *testing.T) {
	tests := map[string]buildSimTest{
		"valid config": {
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Apply: []ApplyConfig{
						{Selector: "wizard", Template: `class {{.Class}}`},
					},
				},
			},
		},
		"empty config": {
			invalid: true,
			cfg:     Config{},
		},
		"config rule->selector not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "",
					Tags:     "-unknown",
					Apply: []ApplyConfig{
						{Selector: "wizard", Template: `class {{.Class}}`},
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
					Apply: []ApplyConfig{
						{Selector: "wizard", Template: `class {{.Class}}`},
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
					Apply: []ApplyConfig{
						{Selector: "wizard", Template: `class {{.Class}}`},
					},
				},
			},
		},
		"config rule->apply not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
				},
			},
		},
		"config rule->apply->selector not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Apply: []ApplyConfig{
						{Selector: "", Template: `class {{.Class}}`},
					},
				},
			},
		},
		"config rule->apply->selector bad syntax": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Apply: []ApplyConfig{
						{Selector: "!", Template: `class {{.Class}}`},
					},
				},
			},
		},
		"config rule->apply->template not set": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Apply: []ApplyConfig{
						{Selector: "wizard", Template: ""},
					},
				},
			},
		},
		"config rule->apply->template missingkey (unknown func)": {
			invalid: true,
			cfg: Config{
				{
					Selector: "unknown",
					Tags:     "-unknown",
					Apply: []ApplyConfig{
						{Selector: "wizard", Template: `class {{error .Class}}`},
					},
				},
			},
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

func TestManager_Build(t *testing.T) {
	tests := map[string]buildSimTest{
		"4 rule service": {
			cfg: Config{
				{
					Selector: "class",
					Tags:     "built",
					Apply: []ApplyConfig{
						{Selector: "*", Template: `Class: {{.Class}}`},
					},
				},
				{
					Selector: "race",
					Tags:     "built",
					Apply: []ApplyConfig{
						{Selector: "*", Template: `Race: {{.Race}}`},
					},
				},
				{
					Selector: "level",
					Tags:     "built",
					Apply: []ApplyConfig{
						{Selector: "*", Template: `Level: {{.Level}}`},
					},
				},
				{
					Selector: "full",
					Tags:     "built",
					Apply: []ApplyConfig{
						{Selector: "*", Template: `Class: {{.Class}}, Race: {{.Race}}, Level: {{.Level}}`},
					},
				},
			},
			values: []buildSimTestValue{
				{
					desc: "1st rule match",
					target: mockTarget{
						tag:   model.Tags{"class": {}},
						Class: "fighter", Race: "orc", Level: 9001,
					},
					wantCfgs: []model.Config{
						{Conf: "Class: fighter", Tags: model.Tags{"built": {}}},
					},
				},
				{
					desc: "1st, 2nd rules match",
					target: mockTarget{
						tag:   model.Tags{"class": {}, "race": {}},
						Class: "fighter", Race: "orc", Level: 9001,
					},
					wantCfgs: []model.Config{
						{Conf: "Class: fighter", Tags: model.Tags{"built": {}}},
						{Conf: "Race: orc", Tags: model.Tags{"built": {}}},
					},
				},
				{
					desc: "1st, 2nd, 3rd rules match",
					target: mockTarget{
						tag:   model.Tags{"class": {}, "race": {}, "level": {}},
						Class: "fighter", Race: "orc", Level: 9001,
					},
					wantCfgs: []model.Config{
						{Conf: "Class: fighter", Tags: model.Tags{"built": {}}},
						{Conf: "Race: orc", Tags: model.Tags{"built": {}}},
						{Conf: "Level: 9001", Tags: model.Tags{"built": {}}},
					},
				},
				{
					desc: "all rules match",
					target: mockTarget{
						tag:   model.Tags{"class": {}, "race": {}, "level": {}, "full": {}},
						Class: "fighter", Race: "orc", Level: 9001,
					},
					wantCfgs: []model.Config{
						{Conf: "Class: fighter", Tags: model.Tags{"built": {}}},
						{Conf: "Race: orc", Tags: model.Tags{"built": {}}},
						{Conf: "Level: 9001", Tags: model.Tags{"built": {}}},
						{Conf: "Class: fighter, Race: orc, Level: 9001", Tags: model.Tags{"built": {}}},
					},
				},
			},
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

func TestRule_Build(t *testing.T) {
	tests := map[string]buildSimTest{
		"simple rule": {
			cfg: Config{
				{
					Selector: "build",
					Tags:     "built",
					Apply: []ApplyConfig{
						{Selector: "human", Template: `Class: {{.Class}}, Race: {{.Race}}, Level: {{.Level}}`},
						{Selector: "missingkey", Template: `{{.Name}}`},
					},
				},
			},
			values: []buildSimTestValue{
				{
					desc: "not match rule selector",
					target: mockTarget{
						tag:   model.Tags{"nothing": {}},
						Class: "fighter", Race: "orc", Level: 9001,
					},
				},
				{
					desc: "not match rule match selector",
					target: mockTarget{
						tag:   model.Tags{"build": {}},
						Class: "fighter", Race: "orc", Level: 9001,
					},
				},
				{
					desc: "match everything",
					target: mockTarget{
						tag:   model.Tags{"build": {}, "human": {}},
						Class: "fighter", Race: "human", Level: 9001,
					},
					wantCfgs: []model.Config{
						{Conf: "Class: fighter, Race: human, Level: 9001", Tags: model.Tags{"built": {}}},
					},
				},
				{
					desc: "missingkey error",
					target: mockTarget{
						tag:   model.Tags{"build": {}, "missingkey": {}},
						Class: "fighter", Race: "human", Level: 9001,
					},
				},
			},
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim.run(t) })
	}
}

type mockTarget struct {
	tag   model.Tags
	Class string
	Race  string
	Level int
}

func (m mockTarget) Tags() model.Tags { return m.tag }
func (mockTarget) Hash() uint64       { return 0 }
func (mockTarget) TUID() string       { return "" }

func (m mockTarget) String() string {
	return fmt.Sprintf("Class: %s, Race: %s, Level: %d, Tags: %s", m.Class, m.Race, m.Level, m.Tags())
}
