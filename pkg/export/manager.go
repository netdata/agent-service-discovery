package export

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/netdata/sd/pkg/model"

	"github.com/mattn/go-isatty"
)

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())

type (
	Config struct {
		File   []FileConfig
		Stdout StdoutConfig
	}
	FileConfig struct {
		Selector string
		Filename string
	}
	StdoutConfig struct {
		Selector string
	}
)

type (
	exporter interface {
		Export(ctx context.Context, out <-chan []model.Config)
	}
	Manager struct {
		exporters []exporter
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	var mgr Manager
	if err := mgr.registerExporters(cfg); err != nil {
		return nil, err
	}

	return &mgr, nil
}

func (m *Manager) registerExporters(cfg Config) error {
	m.exporters = m.exporters[:0]

	for _, fileConf := range cfg.File {
		sr, err := model.ParseSelector(fileConf.Selector)
		if err != nil {
			return err
		}

		e := &File{sr: sr, file: fileConf.Filename}
		m.exporters = append(m.exporters, e)
	}

	if isTerminal {
		sr, err := model.ParseSelector(cfg.Stdout.Selector)
		if err != nil {
			return err
		}

		e := &Stdout{sr: sr}
		m.exporters = append(m.exporters, e)
	}
	return nil
}

func (m *Manager) Export(ctx context.Context, out <-chan []model.Config) {
	var wg sync.WaitGroup
	outs := make([]chan<- []model.Config, 0, len(m.exporters))

	for _, e := range m.exporters {
		eOut := make(chan []model.Config)
		outs = append(outs, eOut)

		wg.Add(1)
		go func(e exporter) {
			defer wg.Done()
			e.Export(ctx, eOut)
		}(e)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.exportLoop(ctx, out, outs)
	}()

	wg.Wait()
	<-ctx.Done()
}

func (m Manager) exportLoop(ctx context.Context, out <-chan []model.Config, outs []chan<- []model.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		case cfgs := <-out:
			for _, eOut := range outs {
				eOut <- cfgs
			}
		}
	}
}

func validateConfig(cfg Config) error {
	if len(cfg.File) == 0 && !isTerminal {
		return errors.New("empty config")
	}

	seen := make(map[string]struct{})
	for i, fileCfg := range cfg.File {
		if fileCfg.Selector == "" {
			return fmt.Errorf("'file->selector' not set [%d]", i+1)
		}
		if fileCfg.Filename == "" {
			return fmt.Errorf("'file->filename' not set [%d]", i+1)
		}
		if _, ok := seen[fileCfg.Filename]; ok {
			return fmt.Errorf("duplicate filename: '%s'", fileCfg.Filename)
		}
		seen[fileCfg.Filename] = struct{}{}
	}
	return nil
}
