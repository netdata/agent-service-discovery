package config

import (
	"github.com/netdata/sd/pipeline/build"
	"github.com/netdata/sd/pipeline/discovery"
	"github.com/netdata/sd/pipeline/export"
	"github.com/netdata/sd/pipeline/tag"

	"github.com/ilyam8/hashstructure"
)

type Config struct {
	Pipeline *PipelineConfig
	Source   string
}

type PipelineConfig struct {
	Name      string           `yaml:"name"`
	Discovery discovery.Config `yaml:"discovery"`
	Tag       tag.Config       `yaml:"tag"`
	Build     build.Config     `yaml:"build"`
	Export    export.Config    `yaml:"export"`
}

func (c PipelineConfig) Hash() uint64 { hash, _ := hashstructure.Hash(c, nil); return hash }
