package pipeline

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/netdata/sd/pipeline/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pipelineSim struct {
	discoveredGroups   []model.Group
	expectedTag        []model.Target
	expectedBuild      []model.Target
	expectedExport     []model.Config
	expectedCacheItems int
}

func (sim pipelineSim) run(t *testing.T) {
	require.NotEmpty(t, sim.discoveredGroups)

	discoverer := &mockDiscoverer{send: sim.discoveredGroups}
	tagger := &mockTagger{}
	builder := &mockBuilder{}
	exporter := &mockExporter{}

	p := New(discoverer, tagger, builder, exporter)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() { defer wg.Done(); p.Run(ctx) }()

	time.Sleep(time.Second)
	cancel()
	wg.Wait()

	sortStaleConfigs(sim.expectedExport)
	sortStaleConfigs(exporter.seen)

	assert.Equal(t, sim.expectedTag, tagger.seen)
	assert.Equal(t, sim.expectedBuild, builder.seen)
	assert.Equal(t, sim.expectedExport, exporter.seen)
	if sim.expectedCacheItems >= 0 {
		assert.Equal(t, sim.expectedCacheItems, len(p.cache))
	}
}

func sortStaleConfigs(cfgs []model.Config) {
	sort.Slice(cfgs, func(i, j int) bool {
		return cfgs[i].Stale && cfgs[j].Stale && cfgs[i].Conf < cfgs[j].Conf
	})
}
