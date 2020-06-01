package build

import (
	"fmt"
	"testing"

	"github.com/netdata/sd/pipeline/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	buildSim struct {
		cfg     Config
		invalid bool
		inputs  []buildSimInput
	}
	buildSimInput struct {
		desc         string
		target       mockTarget
		expectedCfgs []model.Config
	}
)

func (sim buildSim) run(t *testing.T) {
	mgr, err := New(sim.cfg)

	if sim.invalid {
		require.Error(t, err)
		return
	}

	require.NoError(t, err)
	require.NotNil(t, mgr)

	if len(sim.inputs) == 0 {
		return
	}

	for i, input := range sim.inputs {
		name := fmt.Sprintf("input:'%s'[%d], target:'%s', expected configs:'%v'",
			input.desc, i+1, input.target, input.expectedCfgs)

		actual := mgr.Build(input.target)
		assert.Equalf(t, input.expectedCfgs, actual, name)
	}
}
