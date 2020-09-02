package build

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"

	"github.com/netdata/sd/pipeline/model"
	"github.com/netdata/sd/pkg/funcmap"
	"github.com/netdata/sd/pkg/log"

	"github.com/rs/zerolog"
)

type (
	Manager struct {
		rules []*buildRule
		buf   bytes.Buffer
		log   zerolog.Logger
	}
	buildRule struct {
		name  string
		id    int
		sr    model.Selector
		tags  model.Tags
		apply []*ruleApply
	}
	ruleApply struct {
		id   int
		sr   model.Selector
		tags model.Tags
		tmpl *template.Template
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("build manager config validation: %v", err)
	}
	mgr, err := initManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("build manager initialization: %v", err)
	}
	return mgr, nil
}

func (m *Manager) Build(target model.Target) (configs []model.Config) {
	for _, rule := range m.rules {
		if !rule.sr.Matches(target.Tags()) {
			continue
		}

		for _, apply := range rule.apply {
			if !apply.sr.Matches(target.Tags()) {
				continue
			}

			m.buf.Reset()
			if err := apply.tmpl.Execute(&m.buf, target); err != nil {
				m.log.Warn().Err(err).Msgf("failed to execute rule apply '%d/%d' on target '%s'",
					rule.id, apply.id, target.TUID())
				continue
			}

			cfg := model.Config{
				Tags: model.NewTags(),
				Conf: m.buf.String(),
			}

			cfg.Tags.Merge(rule.tags)
			cfg.Tags.Merge(apply.tags)
			configs = append(configs, cfg)
		}
	}
	if len(configs) > 0 {
		m.log.Info().Msgf("built %d config(s) for target '%s'", len(configs), target.TUID())
	}
	return configs
}

func initManager(conf Config) (*Manager, error) {
	if len(conf) == 0 {
		return nil, errors.New("empty config")
	}
	mgr := &Manager{
		log: log.New("build manager"),
	}

	for i, cfg := range conf {
		rule := buildRule{id: i + 1, name: cfg.Name}
		if sr, err := model.ParseSelector(cfg.Selector); err != nil {
			return nil, err
		} else {
			rule.sr = sr
		}

		if tags, err := model.ParseTags(cfg.Tags); err != nil {
			return nil, err
		} else {
			rule.tags = tags
		}

		for i, cfg := range cfg.Apply {
			apply := ruleApply{id: i + 1}
			if sr, err := model.ParseSelector(cfg.Selector); err != nil {
				return nil, err
			} else {
				apply.sr = sr
			}

			if tags, err := model.ParseTags(cfg.Tags); err != nil {
				return nil, err
			} else {
				apply.tags = tags
			}

			if tmpl, err := parseTemplate(cfg.Template); err != nil {
				return nil, err
			} else {
				apply.tmpl = tmpl
			}

			rule.apply = append(rule.apply, &apply)
		}
		mgr.rules = append(mgr.rules, &rule)
	}
	return mgr, nil
}

func parseTemplate(line string) (*template.Template, error) {
	return template.New("root").
		Option("missingkey=error").
		Funcs(funcmap.FuncMap).
		Parse(line)
}
