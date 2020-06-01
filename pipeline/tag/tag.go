package tag

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/netdata/sd/pipeline/model"
	"github.com/netdata/sd/pkg/log"

	"github.com/rs/zerolog"
)

type (
	Manager struct {
		rules []*tagRule
		buf   bytes.Buffer
		log   zerolog.Logger
	}
	tagRule struct {
		name  string
		id    int
		sr    model.Selector
		tags  model.Tags
		match []*ruleMatch
	}
	ruleMatch struct {
		id   int
		sr   model.Selector
		tags model.Tags
		cond *template.Template
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("tag manager config validation: %v", err)
	}
	mgr, err := initManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("tag manager initialization: %v", err)
	}
	return mgr, nil
}

func (m *Manager) Tag(target model.Target) {
	for _, rule := range m.rules {
		if !rule.sr.Matches(target.Tags()) {
			continue
		}

		for _, match := range rule.match {
			if !match.sr.Matches(target.Tags()) {
				continue
			}

			m.buf.Reset()
			if err := match.cond.Execute(&m.buf, target); err != nil {
				m.log.Warn().Err(err).Msgf("failed to execute rule match '%d/%d' on target '%s'",
					rule.id, match.id, target.TUID())
				continue
			}
			if strings.TrimSpace(m.buf.String()) != "true" {
				continue
			}

			target.Tags().Merge(rule.tags)
			target.Tags().Merge(match.tags)
			m.log.Debug().Msgf("matched target '%s', tags: %s", target.TUID(), target.Tags())
		}
	}
}

func initManager(conf Config) (*Manager, error) {
	if len(conf) == 0 {
		return nil, errors.New("empty config")
	}

	mgr := &Manager{
		rules: nil,
		log:   log.New("tag manager"),
	}
	for i, cfg := range conf {
		rule := tagRule{id: i + 1, name: cfg.Name}
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

		for i, cfg := range cfg.Match {
			match := ruleMatch{id: i + 1}
			if sr, err := model.ParseSelector(cfg.Selector); err != nil {
				return nil, err
			} else {
				match.sr = sr
			}

			if tags, err := model.ParseTags(cfg.Tags); err != nil {
				return nil, err
			} else {
				match.tags = tags
			}

			if tmpl, err := parseTemplate(cfg.Cond); err != nil {
				return nil, err
			} else {
				match.cond = tmpl
			}

			rule.match = append(rule.match, &match)
		}
		mgr.rules = append(mgr.rules, &rule)
	}
	return mgr, nil
}

func parseTemplate(line string) (*template.Template, error) {
	return template.New("root").
		Option("missingkey=error").
		Funcs(condTmplFuncMap).
		Parse(line)
}
