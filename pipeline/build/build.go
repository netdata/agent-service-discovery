package build

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"

	"github.com/netdata/sd/pipeline/model"
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

type (
	builder interface {
		Build(model.Target) []model.Config
	}
	Manager struct {
		builders []builder
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("build service config validation: %v", err)
	}
	mgr, err := initManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("build service initialization: %v", err)
	}
	return mgr, nil
}

func (m Manager) Build(target model.Target) []model.Config {
	var configs []model.Config
	for _, b := range m.builders {
		configs = append(configs, b.Build(target)...)
	}
	return configs
}

type Rule struct {
	name  string
	sr    model.Selector
	tags  model.Tags
	apply []*RuleApply

	buf bytes.Buffer
}
type RuleApply struct {
	sr   model.Selector
	tags model.Tags
	tmpl *template.Template
}

func (r *Rule) Build(target model.Target) []model.Config {
	if !r.sr.Matches(target.Tags()) {
		return nil
	}

	var configs []model.Config
	for _, apply := range r.apply {
		if !apply.sr.Matches(target.Tags()) {
			continue
		}
		r.buf.Reset()
		if err := apply.tmpl.Execute(&r.buf, target); err != nil {
			continue
		}
		cfg := model.Config{
			Tags: model.NewTags(),
			Conf: r.buf.String(),
		}
		cfg.Tags.Merge(r.tags)
		cfg.Tags.Merge(apply.tags)
		configs = append(configs, cfg)
	}
	return configs
}

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

		for ii, applyCfg := range ruleCfg.Apply {
			if applyCfg.Selector == "" {
				return fmt.Errorf("'rule->apply->selector' not set (rule %s[%d]/apply [%d])", ruleCfg.Name, i+1, ii+1)
			}
			if applyCfg.Template == "" {
				return fmt.Errorf("'rule->apply->template' not set (rule %s[%d]/apply [%d])", ruleCfg.Name, i+1, ii+1)
			}
		}
	}
	return nil
}

func initManager(conf Config) (*Manager, error) {
	if len(conf) == 0 {
		return nil, errors.New("empty config")
	}

	var mgr Manager
	for _, ruleCfg := range conf {
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

		for _, applyConf := range ruleCfg.Apply {
			var apply RuleApply
			sr, err := model.ParseSelector(applyConf.Selector)
			if err != nil {
				return nil, err
			}
			apply.sr = sr

			tags, err := model.ParseTags(applyConf.Tags)
			if err != nil {
				return nil, err
			}
			apply.tags = tags

			tmpl, err := parseTemplate(applyConf.Template)
			if err != nil {
				return nil, err
			}
			apply.tmpl = tmpl

			rule.apply = append(rule.apply, &apply)
		}

		mgr.builders = append(mgr.builders, &rule)
	}
	return &mgr, nil
}

func parseTemplate(line string) (*template.Template, error) {
	return template.New("root").
		Option("missingkey=error").
		Parse(line)
}
