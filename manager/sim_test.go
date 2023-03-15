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

var lock = &sync.Mutex{}

type runSim struct {
	configs            []config.Config
	expectedBeforeStop []*mockPipeline
	expectedAfterStop  []*mockPipeline
}

func (sim runSim) run(t *testing.T) {
	provider := &mockProvider{
		cfgs: sim.configs,
		ch:   make(chan []config.Config),
	}
	fact := &mockFactory{}
	mgr := New(provider)
	mgr.factory = fact

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() { defer wg.Done(); mgr.Run(ctx) }()
	time.Sleep(time.Second)

	lock.Lock()
	assert.Equalf(t, sim.expectedBeforeStop, fact.created, "before stop")
	lock.Unlock()

	cancel()
	wg.Wait()

	lock.Lock()
	assert.Equalf(t, sim.expectedAfterStop, fact.created, "after stop")
	lock.Unlock()
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
