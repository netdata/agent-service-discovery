package manager

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/netdata/sd/manager/config"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {

}

func TestManager_Run(t *testing.T) {
	tests := map[string]struct {
		configs            []config.Config
		expectedBeforeStop []*mockPipeline
		expectedAfterStop  []*mockPipeline
	}{
		"add pipeline": {
			configs: []config.Config{
				{
					Pipeline: &config.PipelineConfig{Name: "name"},
					Source:   "source",
				},
			},
			expectedBeforeStop: []*mockPipeline{{name: "name", started: true, stopped: false}},
			expectedAfterStop:  []*mockPipeline{{name: "name", started: true, stopped: true}},
		},
		"remove pipeline": {
			configs: []config.Config{
				{
					Pipeline: &config.PipelineConfig{Name: "name"},
					Source:   "source",
				},
				{
					Source: "source",
				},
			},
			expectedBeforeStop: []*mockPipeline{
				{name: "name", started: true, stopped: true},
			},
			expectedAfterStop: []*mockPipeline{
				{name: "name", started: true, stopped: true},
			},
		},
		"several equal configs": {
			configs: []config.Config{
				{
					Pipeline: &config.PipelineConfig{Name: "name"},
					Source:   "source",
				},
				{
					Pipeline: &config.PipelineConfig{Name: "name"},
					Source:   "source",
				},
				{
					Pipeline: &config.PipelineConfig{Name: "name"},
					Source:   "source",
				},
			},
			expectedBeforeStop: []*mockPipeline{
				{name: "name", started: true, stopped: false},
			},
			expectedAfterStop: []*mockPipeline{
				{name: "name", started: true, stopped: true},
			},
		},
		"restart pipeline (same source, different config)": {
			configs: []config.Config{
				{
					Pipeline: &config.PipelineConfig{Name: "name1"},
					Source:   "source",
				},
				{
					Pipeline: &config.PipelineConfig{Name: "name2"},
					Source:   "source",
				},
			},
			expectedBeforeStop: []*mockPipeline{
				{name: "name1", started: true, stopped: true},
				{name: "name2", started: true, stopped: false},
			},
			expectedAfterStop: []*mockPipeline{
				{name: "name1", started: true, stopped: true},
				{name: "name2", started: true, stopped: true},
			},
		},
		"invalid pipeline config": {
			configs: []config.Config{
				{
					Pipeline: &config.PipelineConfig{Name: "invalid"},
					Source:   "source",
				},
			},
		},
		"handle invalid config for running pipeline": {
			configs: []config.Config{
				{
					Pipeline: &config.PipelineConfig{Name: "name"},
					Source:   "source",
				},
				{
					Pipeline: &config.PipelineConfig{Name: "invalid"},
					Source:   "source",
				},
			},
			expectedBeforeStop: []*mockPipeline{
				{name: "name", started: true, stopped: false},
			},
			expectedAfterStop: []*mockPipeline{
				{name: "name", started: true, stopped: true},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			provider := &mockProvider{cfgs: test.configs, ch: make(chan []config.Config)}
			fact := &mockFactory{}
			mgr := New(provider)
			mgr.factory = fact

			var wg sync.WaitGroup
			ctx, cancel := context.WithCancel(context.Background())

			wg.Add(1)
			go func() { defer wg.Done(); mgr.Run(ctx) }()
			time.Sleep(time.Second)

			lock.Lock()
			assert.Equalf(t, test.expectedBeforeStop, fact.created, "before stop")
			lock.Unlock()
			cancel()
			wg.Wait()

			assert.Equalf(t, test.expectedAfterStop, fact.created, "after stop")
		})
	}
}

type (
	mockProvider struct {
		cfgs []config.Config
		ch   chan []config.Config
	}
	mockPipeline struct {
		name    string
		started bool
		stopped bool
	}
	mockFactory struct {
		created []*mockPipeline
	}
)

func (m mockProvider) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
	case m.ch <- m.cfgs:
	}
	<-ctx.Done()
}

func (m mockProvider) Configs() chan []config.Config {
	return m.ch
}

var lock = &sync.Mutex{}

func (m *mockPipeline) Run(ctx context.Context) {
	lock.Lock()
	m.started = true
	lock.Unlock()
	defer func() { lock.Lock(); defer lock.Unlock(); m.stopped = true }()
	<-ctx.Done()
}

func (m *mockFactory) create(cfg config.PipelineConfig) (sdPipeline, error) {
	lock.Lock()
	defer lock.Unlock()

	if cfg.Name == "invalid" {
		return nil, errors.New("mock factory error")
	}
	p := &mockPipeline{name: cfg.Name}
	m.created = append(m.created, p)
	return p, nil
}
