package discovery

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/netdata/sd/pipeline/discovery/kubernetes"
	"github.com/netdata/sd/pipeline/model"
	"github.com/netdata/sd/pkg/log"

	"github.com/rs/zerolog"
)

type Config struct {
	K8S []kubernetes.Config `yaml:"k8s"`
}

func validateConfig(cfg Config) error {
	if len(cfg.K8S) == 0 {
		return errors.New("empty config")
	}
	return nil
}

type (
	discoverer interface {
		Discover(ctx context.Context, in chan<- []model.Group)
	}
	Manager struct {
		discoverers []discoverer
		send        chan struct{}
		sendEvery   time.Duration
		cache       *cache
		log         zerolog.Logger
	}
)

func New(cfg Config) (*Manager, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	mgr := &Manager{
		send:        make(chan struct{}, 1),
		sendEvery:   5 * time.Second,
		discoverers: make([]discoverer, 0),
		cache:       newCache(),
		log:         log.New("discovery manager"),
	}
	if err := mgr.registerDiscoverers(cfg); err != nil {
		return nil, err
	}

	mgr.log.Info().Msgf("registered: %v", mgr.discoverers)
	return mgr, nil
}

func (m *Manager) registerDiscoverers(conf Config) error {
	for _, cfg := range conf.K8S {
		d, err := kubernetes.NewDiscovery(cfg)
		if err != nil {
			return err
		}
		m.discoverers = append(m.discoverers, d)
	}
	return nil
}

func (m *Manager) Discover(ctx context.Context, in chan<- []model.Group) {
	m.log.Info().Msg("instance is started")
	defer m.log.Info().Msg("instance is stopped")

	var wg sync.WaitGroup

	for _, d := range m.discoverers {
		wg.Add(1)
		go func(d discoverer) { defer wg.Done(); m.runDiscoverer(ctx, d) }(d)
	}

	wg.Add(1)
	go func() { defer wg.Done(); m.run(ctx, in) }()

	wg.Wait()
	<-ctx.Done()
}

func (m *Manager) runDiscoverer(ctx context.Context, d discoverer) {
	updates := make(chan []model.Group)
	go d.Discover(ctx, updates)

	for {
		select {
		case <-ctx.Done():
			return
		case groups, ok := <-updates:
			if !ok {
				return
			}
			func() {
				m.cache.mu.Lock()
				defer m.cache.mu.Unlock()

				m.cache.update(groups)
				m.triggerSend()
			}()
		}
	}
}

func (m *Manager) run(ctx context.Context, in chan<- []model.Group) {
	tk := time.NewTicker(m.sendEvery)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			select {
			case <-m.send:
				m.trySend(in)
			default:
			}
		}
	}
}

func (m *Manager) trySend(in chan<- []model.Group) {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	select {
	case in <- m.cache.asList():
		m.cache.reset()
	default:
		m.triggerSend()
	}
}

func (m *Manager) triggerSend() {
	select {
	case m.send <- struct{}{}:
	default:
	}
}
