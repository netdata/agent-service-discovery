package tag

import (
	"fmt"
	"testing"

	"github.com/netdata/sd/pipeline/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	tagSim struct {
		cfg     Config
		invalid bool
		inputs  []tagSimInput
	}
	tagSimInput struct {
		desc         string
		target       mockTarget
		expectedTags model.Tags
	}
)

func (sim tagSim) run(t *testing.T) {
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
		name := fmt.Sprintf("input:'%s'[%d], target:'%s', expected tags:'%s'",
			input.desc, i+1, input.target, input.expectedTags)

		mgr.Tag(input.target)
		assert.Equalf(t, input.expectedTags, input.target.Tags(), name)
	}
}
