package export

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/netdata/sd/pkg/model"
)

type File struct {
	sr    model.Selector
	file  string
	cache cache
	dump  bool
	wr    *bufio.Writer
}

func (f *File) Export(ctx context.Context, out <-chan []model.Config) {
	const exportEvery = time.Second * 1

	f.cache = make(cache)
	tk := time.NewTicker(exportEvery)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case cfgs := <-out:
			f.process(cfgs)
		case <-tk.C:
			f.export()
		}
	}
}

func (f *File) process(cfgs []model.Config) {
	for _, cfg := range cfgs {
		if !f.sr.Matches(cfg.Tags) {
			continue
		}
		if changed := f.cache.put(cfg); changed && !f.dump {
			f.dump = true
		}
	}
}

func (f *File) export() {
	if !f.dump || len(f.cache) == 0 {
		return
	}
	fi, err := os.Create(f.file)
	if err != nil {
		return
	}
	defer fi.Close()

	if f.wr == nil {
		f.wr = bufio.NewWriterSize(fi, 4096*4)
	} else {
		f.wr.Reset(fi)
	}

	for cfg := range f.cache {
		_, _ = f.wr.Write([]byte(cfg + "\n"))
	}
	_ = f.wr.Flush()

	f.dump = false
}

type Stdout struct {
	sr    model.Selector
	wr    strings.Builder
	cache cache
	dump  bool
}

func (s *Stdout) Export(ctx context.Context, out <-chan []model.Config) {
	const exportEvery = time.Second * 1

	s.cache = make(cache)
	tk := time.NewTicker(exportEvery)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case cfgs := <-out:
			s.process(cfgs)
		case <-tk.C:
			s.export()
		}
	}
}

func (s *Stdout) process(cfgs []model.Config) {
	for _, cfg := range cfgs {
		if !s.sr.Matches(cfg.Tags) {
			continue
		}
		if changed := s.cache.put(cfg); changed && !s.dump {
			s.dump = true
		}
	}
}

func (s *Stdout) export() {
	if !s.dump || len(s.cache) == 0 {
		return
	}
	s.dump = false
	defer s.wr.Reset()

	header := fmt.Sprintf("-----------------------CONFIGURATIONS(%d)-----------------------\n", len(s.cache))
	s.wr.WriteString(header)
	for cfg := range s.cache {
		s.wr.Write([]byte(cfg + "\n"))
	}
	fmt.Println(s.wr.String())
}
