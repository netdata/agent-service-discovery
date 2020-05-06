package pipeline

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/netdata/sd/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runSimTest struct {
	inGroups       []model.Group
	wantTag        []model.Target
	wantBuild      []model.Target
	wantExport     []model.Config
	wantCacheItems int
}

func (sim runSimTest) run(t *testing.T) {
	require.NotEmpty(t, sim.inGroups)

	discoverer := &mockDiscoverer{send: sim.inGroups}
	tagger := &mockTagger{}
	builder := &mockBuilder{}
	exporter := &mockExporter{}

	p := Pipeline{
		Discoverer: discoverer,
		Tagger:     tagger,
		Builder:    builder,
		Exporter:   exporter,
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() { defer wg.Done(); p.Run(ctx) }()

	time.Sleep(time.Second)
	cancel()
	wg.Wait()

	sortStaleConfigs(sim.wantExport)
	sortStaleConfigs(exporter.seen)

	assert.Equal(t, sim.wantTag, tagger.seen)
	assert.Equal(t, sim.wantBuild, builder.seen)
	assert.Equal(t, sim.wantExport, exporter.seen)
	if sim.wantCacheItems >= 0 {
		assert.Equal(t, sim.wantCacheItems, len(p.cache))
	}
}

func sortStaleConfigs(cfgs []model.Config) {
	sort.Slice(cfgs, func(i, j int) bool {
		return cfgs[i].Stale && cfgs[j].Stale && cfgs[i].Conf < cfgs[j].Conf
	})
}
