package tag

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/netdata/sd/pipeline/model"
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
		Cond     string `yaml:"cond"`     // mandatory
	}
)

type (
	tagger interface {
		Tag(model.Target)
	}
	Manager struct {
		taggers []tagger
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("tag service config validation: %v", err)
	}
	mgr, err := initManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("tag service initialization: %v", err)
	}
	return mgr, nil
}

func (m Manager) Tag(target model.Target) {
	for _, t := range m.taggers {
		t.Tag(target)
	}
}

type (
	Rule struct {
		name  string
		sr    model.Selector
		tags  model.Tags
		match []*RuleMatch
		buf   bytes.Buffer
	}
	RuleMatch struct {
		sr      model.Selector
		tags    model.Tags
		cond    *template.Template
		rawCond string
	}
)

func (r *Rule) Tag(target model.Target) {
	if !r.sr.Matches(target.Tags()) {
		return
	}
	for _, match := range r.match {
		if !match.sr.Matches(target.Tags()) {
			continue
		}
		r.buf.Reset()
		if err := match.cond.Execute(&r.buf, target); err != nil {
			continue
		}
		if strings.TrimSpace(r.buf.String()) != "true" {
			continue
		}
		target.Tags().Merge(r.tags)
		target.Tags().Merge(match.tags)
	}
}

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

		for ii, match := range rule.Match {
			if match.Tags == "" {
				return fmt.Errorf("'rule->match->tags' not set (rule %s[%d]/match [%d])", rule.Name, i+1, ii+1)
			}
			if match.Cond == "" {
				return fmt.Errorf("'rule->match->cond' not set (rule %s[%d]/match [%d])", rule.Name, i+1, ii+1)
			}
		}
	}
	return nil
}

func initManager(cfg Config) (*Manager, error) {
	if len(cfg) == 0 {
		return nil, errors.New("empty config")
	}

	var mgr Manager
	for _, ruleCfg := range cfg {
		var rule Rule
		sr, err := model.ParseSelector(ruleCfg.Selector)
		if err != nil {
			return nil, err
		}
		rule.sr = sr

		tags, err := model.ParseTags(ruleCfg.Tags)
		if err != nil {
			return nil, err
		}
		rule.tags = tags

		for _, matchCfg := range ruleCfg.Match {
			var match RuleMatch
			sr, err := model.ParseSelector(matchCfg.Selector)
			if err != nil {
				return nil, err
			}
			match.sr = sr

			tags, err := model.ParseTags(matchCfg.Tags)
			if err != nil {
				return nil, err
			}
			match.tags = tags

			tmpl, err := parseTemplate(matchCfg.Cond)
			if err != nil {
				return nil, err
			}

			match.cond = tmpl
			match.rawCond = matchCfg.Cond

			rule.match = append(rule.match, &match)
		}

		mgr.taggers = append(mgr.taggers, &rule)
	}
	return &mgr, nil
}

func parseTemplate(line string) (*template.Template, error) {
	return template.New("root").
		Option("missingkey=error").
		Funcs(condTmplFuncMap).
		Parse(line)
}
