package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/netdata/sd/manager/config"
	"github.com/netdata/sd/pipeline"
	"github.com/netdata/sd/pipeline/build"
	"github.com/netdata/sd/pipeline/discovery"
	"github.com/netdata/sd/pipeline/export"
	"github.com/netdata/sd/pipeline/tag"
)

type ConfigProvider interface {
	Run(ctx context.Context)
	Configs() chan []config.Config
}

type (
	Manager struct {
		prov ConfigProvider

		create func(cfg config.PipelineConfig) (*pipeline.Pipeline, error)

		cache     map[string]uint64
		pipelines map[string]func()
	}
)

func New(provider ConfigProvider) *Manager {
	return &Manager{
		prov:      provider,
		create:    newPipeline,
		cache:     make(map[string]uint64),
		pipelines: make(map[string]func()),
	}
}

func (m *Manager) Run(ctx context.Context) {
	defer m.cleanup()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() { defer wg.Done(); m.prov.Run(ctx) }()

	wg.Add(1)
	go func() { defer wg.Done(); m.run(ctx) }()

	wg.Wait()
	<-ctx.Done()
}

func (m *Manager) cleanup() {
	for _, stop := range m.pipelines {
		stop()
	}
}

func (m *Manager) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case cfgs := <-m.prov.Configs():
			for _, cfg := range cfgs {
				select {
				case <-ctx.Done():
					return
				default:
					m.process(ctx, cfg)
				}
			}
		}
	}
}

func (m *Manager) process(ctx context.Context, cfg config.Config) {
	if cfg.Source == "" {
		return
	}

	if cfg.Pipeline == nil {
		delete(m.cache, cfg.Source)
		m.handleRemoveConfig(cfg)
		return
	}

	if hash, ok := m.cache[cfg.Source]; !ok || hash != cfg.Pipeline.Hash() {
		m.cache[cfg.Source] = hash
		m.handleNewConfig(ctx, cfg)
	}
}

func (m *Manager) handleRemoveConfig(cfg config.Config) {
	if stop, ok := m.pipelines[cfg.Source]; ok {
		delete(m.pipelines, cfg.Source)
		fmt.Println("STOP", cfg.Source)
		stop()
	}
}

func (m *Manager) handleNewConfig(ctx context.Context, cfg config.Config) {
	p, err := m.newPipeline(*cfg.Pipeline)
	if err != nil {
		return
	}

	if stop, ok := m.pipelines[cfg.Source]; ok {
		fmt.Println("STOP", cfg.Source)
		stop()
	}
	fmt.Println("START", cfg.Source)

	var wg sync.WaitGroup
	pipelineCtx, cancel := context.WithCancel(ctx)

	wg.Add(1)
	go func() { defer wg.Done(); p.Run(pipelineCtx) }()
	stop := func() { cancel(); wg.Wait() }

	m.pipelines[cfg.Source] = stop
}

func (m *Manager) newPipeline(cfg config.PipelineConfig) (*pipeline.Pipeline, error) {
	return m.create(cfg)
}

func newPipeline(cfg config.PipelineConfig) (*pipeline.Pipeline, error) {
	discoverer, err := discovery.New(cfg.Discovery)
	if err != nil {
		return nil, err
	}
	tagger, err := tag.New(cfg.Tag)
	if err != nil {
		return nil, err
	}
	builder, err := build.New(cfg.Build)
	if err != nil {
		return nil, err
	}
	exporter, err := export.New(cfg.Export)
	if err != nil {
		return nil, err
	}
	return pipeline.New(discoverer, tagger, builder, exporter), nil
}
