package tag

import (
	"errors"
	"fmt"
)

type (
	Config     []RuleConfig // mandatory, at least 1
	RuleConfig struct {
		Name     string        `yaml:"name"`
		Selector string        `yaml:"selector"` // mandatory
		Tags     string        `yaml:"tags"`     // mandatory
		Match    []MatchConfig `yaml:"match"`    // mandatory, at least 1
	}
	MatchConfig struct {
		Selector string `yaml:"selector"` // optional
		Tags     string `yaml:"tags"`     // mandatory
		Expr     string `yaml:"expr"`     // mandatory
	}
)

func validateConfig(cfg Config) error {
	if len(cfg) == 0 {
		return errors.New("empty config, need least 1 rule")
	}
	for i, rule := range cfg {
		if rule.Selector == "" {
			return fmt.Errorf("'rule->selector' not set (rule %s[%d])", rule.Name, i+1)
		}
		if rule.Tags == "" {
			return fmt.Errorf("'rule->tags' not set (rule %s[%d])", rule.Name, i+1)
		}
		if len(rule.Match) == 0 {
			return fmt.Errorf("'rule->match' not set, need at least 1 rule match (rule %s[%d])", rule.Name, i+1)
		}

		for j, match := range rule.Match {
			if match.Tags == "" {
				return fmt.Errorf("'rule->match->tags' not set (rule %s[%d]/match [%d])", rule.Name, i+1, j+1)
			}
			if match.Expr == "" {
				return fmt.Errorf("'rule->match->expr' not set (rule %s[%d]/match [%d])", rule.Name, i+1, j+1)
			}
		}
	}
	return nil
}
