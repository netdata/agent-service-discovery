package pipeline

import (
	"context"
	"testing"

	"github.com/netdata/sd/pipeline/model"

	"github.com/ilyam8/hashstructure"
)

func TestPipeline_Run(t *testing.T) {
	tests := map[string]func() pipelineSim{
		"new group with no targets": func() pipelineSim {
			g1 := mockGroup{source: "s1"}

			sim := pipelineSim{
				discoveredGroups:   []model.Group{g1},
				expectedCacheItems: 0,
			}
			return sim
		},
		"new group with targets": func() pipelineSim {
			t1 := mockTarget{Name: "t1"}
			t2 := mockTarget{Name: "t2"}
			g1 := mockGroup{targets: []model.Target{t1, t2}, source: "s1"}

			sim := pipelineSim{
				discoveredGroups:   []model.Group{g1},
				expectedTag:        []model.Target{t1, t2},
				expectedBuild:      []model.Target{t1, t2},
				expectedExport:     []model.Config{{Conf: "t1"}, {Conf: "t2"}},
				expectedCacheItems: 1,
			}
			return sim
		},
		"existing group with same targets": func() pipelineSim {
			t1 := mockTarget{Name: "t1"}
			t2 := mockTarget{Name: "t2"}
			g1 := mockGroup{targets: []model.Target{t1, t2}, source: "s1"}

			sim := pipelineSim{
				discoveredGroups:   []model.Group{g1, g1},
				expectedTag:        []model.Target{t1, t2},
				expectedBuild:      []model.Target{t1, t2},
				expectedExport:     []model.Config{{Conf: "t1"}, {Conf: "t2"}},
				expectedCacheItems: 1,
			}
			return sim
		},
		"existing group with no targets": func() pipelineSim {
			t1 := mockTarget{Name: "t1"}
			t2 := mockTarget{Name: "t2"}
			g1 := mockGroup{targets: []model.Target{t1, t2}, source: "s1"}
			g2 := mockGroup{source: "s1"}

			sim := pipelineSim{
				discoveredGroups: []model.Group{g1, g2},
				expectedTag:      []model.Target{t1, t2},
				expectedBuild:    []model.Target{t1, t2},
				expectedExport: []model.Config{
					{Conf: "t1"}, {Conf: "t2"}, {Conf: "t1", Stale: true}, {Conf: "t2", Stale: true},
				},
				expectedCacheItems: 0,
			}
			return sim
		},
		"existing group with old and new targets": func() pipelineSim {
			t1 := mockTarget{Name: "t1"}
			t2 := mockTarget{Name: "t2"}
			t3 := mockTarget{Name: "t3"}
			g1 := mockGroup{targets: []model.Target{t1, t2}, source: "s1"}
			g2 := mockGroup{targets: []model.Target{t1, t3}, source: "s1"}

			sim := pipelineSim{
				discoveredGroups: []model.Group{g1, g2},
				expectedTag:      []model.Target{t1, t2, t3},
				expectedBuild:    []model.Target{t1, t2, t3},
				expectedExport: []model.Config{
					{Conf: "t1"}, {Conf: "t2"}, {Conf: "t3"}, {Conf: "t2", Stale: true}},
				expectedCacheItems: 1,
			}
			return sim
		},
		"existing group with new targets only": func() pipelineSim {
			t1 := mockTarget{Name: "t1"}
			t2 := mockTarget{Name: "t2"}
			t3 := mockTarget{Name: "t3"}
			g1 := mockGroup{targets: []model.Target{t1, t2}, source: "s1"}
			g2 := mockGroup{targets: []model.Target{t3}, source: "s1"}

			sim := pipelineSim{
				discoveredGroups: []model.Group{g1, g2},
				expectedTag:      []model.Target{t1, t2, t3},
				expectedBuild:    []model.Target{t1, t2, t3},
				expectedExport: []model.Config{
					{Conf: "t1"}, {Conf: "t2"}, {Conf: "t3"},
					{Conf: "t1", Stale: true}, {Conf: "t2", Stale: true}},
				expectedCacheItems: 1,
			}
			return sim
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim().run(t) })
	}
}

type (
	mockDiscoverer struct {
		send []model.Group
	}
	mockTagger struct {
		seen []model.Target
	}
	mockBuilder struct {
		seen []model.Target
	}
	mockExporter struct {
		seen []model.Config
	}
)

func (d mockDiscoverer) Discover(ctx context.Context, in chan<- []model.Group) {
	select {
	case <-ctx.Done():
	case in <- d.send:
	}
	<-ctx.Done()
}

func (t *mockTagger) Tag(target model.Target) {
	t.seen = append(t.seen, target)
}

func (b *mockBuilder) Build(target model.Target) []model.Config {
	b.seen = append(b.seen, target)
	return []model.Config{{Conf: target.TUID()}}
}

func (e *mockExporter) Export(ctx context.Context, out <-chan []model.Config) {
	select {
	case <-ctx.Done():
	case cfgs := <-out:
		e.seen = append(e.seen, cfgs...)
	}
	<-ctx.Done()
}

type (
	mockGroup struct {
		targets []model.Target
		source  string
	}
	mockTarget struct {
		Name string
	}
)

func (mg mockGroup) Targets() []model.Target { return mg.targets }
func (mg mockGroup) Source() string          { return mg.source }

func (mt mockTarget) Tags() model.Tags { return nil }
func (mt mockTarget) TUID() string     { return mt.Name }
func (mt mockTarget) Hash() uint64     { h, _ := hashstructure.Hash(mt, nil); return h }
