package discovery

import (
	"sync"

	"github.com/netdata/sd/pkg/model"
)

type cache struct {
	mu    sync.RWMutex
	items map[string]model.Group
}

func newCache() *cache {
	return &cache{
		mu:    sync.RWMutex{},
		items: make(map[string]model.Group),
	}
}

func (c *cache) update(groups []model.Group) {
	for _, group := range groups {
		if group != nil {
			c.items[group.Source()] = group
		}
	}
}

func (c *cache) reset() {
	for key := range c.items {
		delete(c.items, key)
	}
}

func (c *cache) asList() []model.Group {
	groups := make([]model.Group, 0, len(c.items))
	for _, group := range c.items {
		groups = append(groups, group)
	}
	return groups
}
