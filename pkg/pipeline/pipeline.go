package pipeline

import (
	"context"
	"sync"

	"github.com/netdata/sd/pkg/model"
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

		configs []model.Config
		cache   cache
	}
	cache      map[string]groupCache // source:hash:configs
	groupCache map[uint64][]model.Config
)

func (p *Pipeline) Run(ctx context.Context) {
	p.cache = make(cache)

	var wg sync.WaitGroup
	discCh := make(chan []model.Group)
	exportCh := make(chan []model.Config)

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Discover(ctx, discCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.processLoop(ctx, discCh, exportCh)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Export(ctx, exportCh)
	}()

	wg.Wait()
	<-ctx.Done()
}

func (p *Pipeline) processLoop(ctx context.Context, discCh chan []model.Group, exportCh chan []model.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		case groups, ok := <-discCh:
			if !ok {
				return
			}
			p.processGroups(groups)
			if len(p.configs) == 0 {
				continue
			}
			exportCh <- p.configs
			p.configs = p.configs[:0:0]
		}
	}
}

func (p *Pipeline) processGroups(groups []model.Group) {
	for _, group := range groups {
		if len(group.Targets()) == 0 {
			p.processEmptyGroup(group)
		} else {
			p.processGroupWithTargets(group)
		}
	}
}

func (p *Pipeline) processEmptyGroup(group model.Group) {
	grpCache, grpExist := p.cache[group.Source()]
	if !grpExist {
		return
	}

	for hash, cfgs := range grpCache {
		delete(grpCache, hash)
		markConfigsStale(cfgs)
		p.configs = append(p.configs, cfgs...)
	}
	delete(p.cache, group.Source())
}

func (p *Pipeline) processGroupWithTargets(group model.Group) {
	grpCache, grpExist := p.cache[group.Source()]
	if !grpExist {
		grpCache = make(map[uint64][]model.Config)
		p.cache[group.Source()] = grpCache
	}

	seen := make(map[uint64]struct{})

	for _, target := range group.Targets() {
		if target == nil {
			continue
		}
		seen[target.Hash()] = struct{}{}

		if _, ok := grpCache[target.Hash()]; ok {
			continue
		}

		p.Tag(target)
		configs := p.Build(target)

		grpCache[target.Hash()] = configs
		p.configs = append(p.configs, configs...)
	}

	if !grpExist || len(p.configs) == 0 {
		return
	}

	for hash, cfgs := range grpCache {
		if _, ok := seen[hash]; ok {
			continue
		}
		delete(grpCache, hash)
		markConfigsStale(cfgs)
		p.configs = append(p.configs, cfgs...)
	}
}

func markConfigsStale(configs []model.Config) {
	for i := range configs {
		configs[i].Stale = true
	}
}
