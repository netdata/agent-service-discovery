package build

import (
	"errors"
	"fmt"
)

type (
	Config     []RuleConfig // mandatory, at least 1
	RuleConfig struct {
		Name     string        `yaml:"name"`     // optional
		Selector string        `yaml:"selector"` // mandatory
		Tags     string        `yaml:"tags"`     // mandatory
		Apply    []ApplyConfig `yaml:"apply"`    // mandatory, at least 1
	}
	ApplyConfig struct {
		Selector string `yaml:"selector"` // mandatory
		Tags     string `yaml:"tags"`     // optional
		Template string `yaml:"template"` // mandatory
	}
)

func validateConfig(cfg Config) error {
	if len(cfg) == 0 {
		return errors.New("empty config, need least 1 rule")
	}
	for i, ruleCfg := range cfg {
		if ruleCfg.Selector == "" {
			return fmt.Errorf("'rule->selector' not set (rule %s[%d])", ruleCfg.Name, i+1)
		}

		if ruleCfg.Tags == "" {
			return fmt.Errorf("'rule->tags' not set (rule %s[%d])", ruleCfg.Name, i+1)
		}
		if len(ruleCfg.Apply) == 0 {
			return fmt.Errorf("'rule->apply' not set (rule %s[%d])", ruleCfg.Name, i+1)
		}

		for j, applyCfg := range ruleCfg.Apply {
			if applyCfg.Selector == "" {
				return fmt.Errorf("'rule->apply->selector' not set (rule %s[%d]/apply [%d])",
					ruleCfg.Name, i+1, j+1)
			}
			if applyCfg.Template == "" {
				return fmt.Errorf("'rule->apply->template' not set (rule %s[%d]/apply [%d])",
					ruleCfg.Name, i+1, j+1)
			}
		}
	}
	return nil
}
