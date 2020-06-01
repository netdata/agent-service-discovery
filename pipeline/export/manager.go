package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/netdata/sd/pipeline/model"
	"github.com/netdata/sd/pkg/log"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())

type (
	Config struct {
		File []FileConfig `yaml:"file"`
	}
	FileConfig struct {
		Selector string `yaml:"selector"`
		Filename string `yaml:"filename"`
	}
)

func validateConfig(conf Config) error {
	if len(conf.File) == 0 && !isTerminal {
		return errors.New("empty config")
	}

	seen := make(map[string]bool)
	for i, cfg := range conf.File {
		if cfg.Selector == "" {
			return fmt.Errorf("'file->selector' not set [%d]", i+1)
		}
		if cfg.Filename == "" {
			return fmt.Errorf("'file->filename' not set [%d]", i+1)
		}
		if seen[cfg.Filename] {
			return fmt.Errorf("duplicate filename: '%s'", cfg.Filename)
		}
		seen[cfg.Filename] = true
	}
	return nil
}

type (
	Manager struct {
		exporters []exporter
		log       zerolog.Logger
	}
	exporter interface {
		Export(ctx context.Context, out <-chan []model.Config)
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	mgr := &Manager{
		log: log.New("export manager"),
	}
	if err := mgr.registerExporters(cfg); err != nil {
		return nil, err
	}

	mgr.log.Info().Msgf("registered: '%v'", mgr.exporters)
	return mgr, nil
}

func (m *Manager) registerExporters(conf Config) error {
	for _, cfg := range conf.File {
		sr, err := model.ParseSelector(cfg.Selector)
		if err != nil {
			return err
		}
		m.exporters = append(m.exporters, NewFile(sr, cfg.Filename))
	}
	if isTerminal {
		m.exporters = append(m.exporters, newStdout())
	}
	return nil
}

func (m *Manager) Export(ctx context.Context, out <-chan []model.Config) {
	m.log.Info().Msg("instance is started")
	defer m.log.Info().Msg("instance is stopped")

	var wg sync.WaitGroup
	outs := make([]chan<- []model.Config, 0, len(m.exporters))

	for _, e := range m.exporters {
		eOut := make(chan []model.Config)
		outs = append(outs, eOut)

		wg.Add(1)
		go func(e exporter) { defer wg.Done(); e.Export(ctx, eOut) }(e)
	}

	wg.Add(1)
	go func() { defer wg.Done(); m.run(ctx, out, outs) }()

	wg.Wait()
	<-ctx.Done()
}

func (m Manager) run(ctx context.Context, out <-chan []model.Config, outs []chan<- []model.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		case cfgs := <-out:
			m.notify(ctx, cfgs, outs)
		}
	}
}

func (m Manager) notify(ctx context.Context, cfgs []model.Config, outs []chan<- []model.Config) {
	for _, out := range outs {
		select {
		case <-ctx.Done():
			return
		case out <- cfgs:
		}
	}
}
