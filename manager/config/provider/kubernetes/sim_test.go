package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/netdata/sd/manager/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/cache"
)

const (
	startDeadline   = time.Second
	collectDeadline = time.Second * 2
)

type runSim struct {
	provider        *Provider
	runAfterSync    func(ctx context.Context)
	expectedConfigs []config.Config
}

func (sim runSim) run(t *testing.T) {
	t.Helper()
	require.NotNil(t, sim.provider)

	collect := make(chan []config.Config)
	go sim.collect(t, collect)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	go sim.provider.Run(ctx)

	select {
	case <-sim.provider.started:
	case <-time.After(startDeadline):
		t.Fatalf("provider '%s' filed to start in %s", *sim.provider, startDeadline)
	}

	synced := cache.WaitForCacheSync(ctx.Done(), sim.provider.inf.HasSynced)
	require.Truef(t, synced, "provider '%s' failed to sync", *sim.provider)

	if sim.runAfterSync != nil {
		sim.runAfterSync(ctx)
	}

	assert.Equal(t, sim.expectedConfigs, <-collect)
}

func (sim runSim) collect(t *testing.T, in chan []config.Config) {
	var configs []config.Config
loop:
	for {
		select {
		case updates := <-sim.provider.Configs():
			if configs = append(configs, updates...); len(configs) >= len(sim.expectedConfigs) {
				break loop
			}
		case <-time.After(collectDeadline):
			t.Logf("provider '%s' timed out after %s, got %d configs, expected %d, some events are skipped",
				*sim.provider, collectDeadline, len(configs), len(sim.expectedConfigs))
			break loop
		}
	}
	in <- configs
}
