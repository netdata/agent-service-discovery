package manager

import (
	"testing"

	"github.com/netdata/sd/manager/config"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	tests := map[string]struct {
		prov ConfigProvider
	}{
		"nil provider":   {prov: nil},
		"valid provider": {prov: &mockProvider{}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.NotNil(t, New(test.prov))
		})
	}
}

func TestManager_Run(t *testing.T) {
	tests := map[string]func() runSim{
		"add pipeline": func() runSim {
			sim := runSim{
				configs: []config.Config{
					prepareConfig("source", "name"),
				},
				expectedBeforeStop: []*mockPipeline{
					{name: "name", started: true, stopped: false},
				},
				expectedAfterStop: []*mockPipeline{
					{name: "name", started: true, stopped: true},
				},
			}
			return sim
		},
		"remove pipeline": func() runSim {
			sim := runSim{
				configs: []config.Config{
					prepareConfig("source", "name"),
					prepareEmptyConfig("source"),
				},
				expectedBeforeStop: []*mockPipeline{
					{name: "name", started: true, stopped: true},
				},
				expectedAfterStop: []*mockPipeline{
					{name: "name", started: true, stopped: true},
				},
			}
			return sim
		},
		"several equal configs": func() runSim {
			sim := runSim{
				configs: []config.Config{
					prepareConfig("source", "name"),
					prepareConfig("source", "name"),
					prepareConfig("source", "name"),
				},
				expectedBeforeStop: []*mockPipeline{
					{name: "name", started: true, stopped: false},
				},
				expectedAfterStop: []*mockPipeline{
					{name: "name", started: true, stopped: true},
				},
			}
			return sim
		},
		"restart pipeline (same source, different config)": func() runSim {
			sim := runSim{
				configs: []config.Config{
					prepareConfig("source", "name1"),
					prepareConfig("source", "name2"),
				},
				expectedBeforeStop: []*mockPipeline{
					{name: "name1", started: true, stopped: true},
					{name: "name2", started: true, stopped: false},
				},
				expectedAfterStop: []*mockPipeline{
					{name: "name1", started: true, stopped: true},
					{name: "name2", started: true, stopped: true},
				},
			}
			return sim
		},
		"invalid pipeline config": func() runSim {
			sim := runSim{
				configs: []config.Config{
					prepareConfig("source", "invalid"),
				},
			}
			return sim
		},
		"handle invalid config for running pipeline": func() runSim {
			sim := runSim{
				configs: []config.Config{
					prepareConfig("source", "name"),
					prepareConfig("source", "invalid"),
				},
				expectedBeforeStop: []*mockPipeline{
					{name: "name", started: true, stopped: false},
				},
				expectedAfterStop: []*mockPipeline{
					{name: "name", started: true, stopped: true},
				},
			}
			return sim
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim().run(t) })
	}
}

func prepareConfig(source, name string) config.Config {
	return config.Config{Pipeline: &config.PipelineConfig{Name: name}, Source: source}
}

func prepareEmptyConfig(source string) config.Config {
	return config.Config{Source: source}
}
