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
		values  []buildSimValue
	}
	buildSimValue struct {
		desc     string
		target   mockTarget
		wantCfgs []model.Config
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

	if len(sim.values) == 0 {
		return
	}

	for i, value := range sim.values {
		name := fmt.Sprintf("test value:'%s'[%d], target:'%s', wantConfigs:'%v'",
			value.desc, i+1, value.target, value.wantCfgs)

		actualCfgs := mgr.Build(value.target)
		assert.Equalf(t, value.wantCfgs, actualCfgs, name)
	}
}
