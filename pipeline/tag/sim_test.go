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
		values  []tagSimValue
	}
	tagSimValue struct {
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

	if len(sim.values) == 0 {
		return
	}

	for i, value := range sim.values {
		name := fmt.Sprintf("test value:'%s'[%d], target:'%s', wantTags:'%s'",
			value.desc, i+1, value.target, value.expectedTags)

		mgr.Tag(value.target)
		assert.Equalf(t, value.expectedTags, value.target.Tags(), name)
	}
}
