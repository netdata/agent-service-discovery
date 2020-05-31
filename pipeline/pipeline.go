package pipeline

import (
	"context"
	"sync"

	"github.com/netdata/sd/pipeline/model"
)

type Discoverer interface {
	Discover(ctx context.Context, in chan<- []model.Group)
}

type Tagger interface {
	Tag(model.Target)
}

type Builder interface {
	Build(model.Target) []model.Config
}

type Exporter interface {
	Export(ctx context.Context, out <-chan []model.Config)
}

type (
	Pipeline struct {
		Discoverer
		Tagger
		Builder
		Exporter

		cache cache
	}
	cache      map[string]groupCache // source:hash:configs
	groupCache map[uint64][]model.Config
)

func New(discoverer Discoverer, tagger Tagger, builder Builder, exporter Exporter) *Pipeline {
	return &Pipeline{
		Discoverer: discoverer,
		Tagger:     tagger,
		Builder:    builder,
		Exporter:   exporter,
		cache:      make(cache),
	}
}

func (p *Pipeline) Run(ctx context.Context) {
	var wg sync.WaitGroup
	disc := make(chan []model.Group)
	exp := make(chan []model.Config)

	wg.Add(1)
	go func() { defer wg.Done(); p.Discover(ctx, disc) }()

	wg.Add(1)
	go func() { defer wg.Done(); p.processLoop(ctx, disc, exp) }()

	wg.Add(1)
	go func() { defer wg.Done(); p.Export(ctx, exp) }()

	wg.Wait()
	<-ctx.Done()
}

func (p *Pipeline) processLoop(ctx context.Context, disc chan []model.Group, export chan []model.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		case groups := <-disc:
			select {
			case <-ctx.Done():
			case export <- p.process(groups):
			}
		}
	}
}

func (p *Pipeline) process(groups []model.Group) (configs []model.Config) {
	for _, group := range groups {
		if len(group.Targets()) == 0 {
			configs = append(configs, p.handleEmpty(group)...)
		} else {
			configs = append(configs, p.handleNotEmpty(group)...)
		}
	}
	return configs
}

func (p *Pipeline) handleEmpty(group model.Group) (configs []model.Config) {
	grpCache, exist := p.cache[group.Source()]
	if !exist {
		return
	}
	delete(p.cache, group.Source())

	for hash, cfgs := range grpCache {
		delete(grpCache, hash)
		configs = append(configs, cfgs...)
	}

	return stale(configs)
}

func (p *Pipeline) handleNotEmpty(group model.Group) (configs []model.Config) {
	grpCache, exist := p.cache[group.Source()]
	if !exist {
		grpCache = make(map[uint64][]model.Config)
		p.cache[group.Source()] = grpCache
	}

	seen := make(map[uint64]bool)
	for _, target := range group.Targets() {
		if target == nil {
			continue
		}
		seen[target.Hash()] = true

		if _, ok := grpCache[target.Hash()]; ok {
			continue
		}

		p.Tag(target)
		cfgs := p.Build(target)

		grpCache[target.Hash()] = cfgs
		configs = append(configs, cfgs...)
	}

	if !exist {
		return
	}

	for hash, cfgs := range grpCache {
		if !seen[hash] {
			delete(grpCache, hash)
			configs = append(configs, stale(cfgs)...)
		}
	}
	return configs
}

func stale(configs []model.Config) []model.Config {
	for i := range configs {
		configs[i].Stale = true
	}
	return configs
}
