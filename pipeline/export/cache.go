package export

import (
	"github.com/netdata/sd/pipeline/model"
)

type cache map[string]int

func (c cache) put(cfg model.Config) (changed bool) {
	count, ok := c[cfg.Conf]
	// add
	if !cfg.Stale {
		c[cfg.Conf]++
		return !ok
	}
	// remove
	if !ok {
		return false
	}
	if count--; count > 0 {
		c[cfg.Conf] = count
		return false
	}
	delete(c, cfg.Conf)
	return true
}
