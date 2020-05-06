package discovery

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/netdata/sd/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type discoverySimTest struct {
	mgr            *Manager
	collectDelay   time.Duration
	expectedGroups []model.Group
}

func (sim discoverySimTest) run(t *testing.T) {
	t.Helper()
	require.NotNil(t, sim.mgr)

	in, out := make(chan []model.Group), make(chan []model.Group)
	go sim.collectGroups(t, in, out)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sim.mgr.Discover(ctx, in)

	actualGroups := <-out

	sortGroups(sim.expectedGroups)
	sortGroups(actualGroups)

	assert.Equal(t, sim.expectedGroups, actualGroups)
}

func (sim discoverySimTest) collectGroups(t *testing.T, in, out chan []model.Group) {
	time.Sleep(sim.collectDelay)

	timeout := sim.mgr.sendEvery + time.Second*2
	var groups []model.Group
loop:
	for {
		select {
		case inGroups := <-in:
			if groups = append(groups, inGroups...); len(groups) >= len(sim.expectedGroups) {
				break loop
			}
		case <-time.After(timeout):
			t.Logf("discovery %s timed out after %s, got %d groups, expected %d, some events are skipped",
				sim.mgr.discoverers, timeout, len(groups), len(sim.expectedGroups))
			break loop
		}
	}
	out <- groups
}

func sortGroups(groups []model.Group) {
	if len(groups) == 0 {
		return
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Source() < groups[j].Source() })
}
