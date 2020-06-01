package file

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/netdata/sd/manager/config"
	"github.com/netdata/sd/pkg/log"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

type (
	Provider struct {
		paths        []string
		watcher      *fsnotify.Watcher
		cache        cache
		refreshEvery time.Duration
		configCh     chan []config.Config
		log          zerolog.Logger
	}
	cache map[string]time.Time
)

func NewProvider(paths []string) *Provider {
	return &Provider{
		paths:        paths,
		cache:        make(cache),
		refreshEvery: time.Minute,
		configCh:     make(chan []config.Config),
		log:          log.New("file config provider"),
	}
}

func (c cache) lookup(path string) (time.Time, bool) { v, ok := c[path]; return v, ok }
func (c cache) has(path string) bool                 { _, ok := c.lookup(path); return ok }
func (c cache) remove(path string)                   { delete(c, path) }
func (c cache) put(path string, modTime time.Time)   { c[path] = modTime }

func (p *Provider) Configs() chan []config.Config {
	return p.configCh
}

func (p *Provider) Run(ctx context.Context) {
	p.log.Info().Msg("instance is started")
	defer p.log.Info().Msg("instance is stopped")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		p.log.Error().Err(err).Msg("unable to initialize fsnotify watcher")
		return
	}

	p.watcher = watcher
	defer p.stop()
	p.refresh(ctx)

	tk := time.NewTicker(p.refreshEvery)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			p.refresh(ctx)
		case event := <-p.watcher.Events:
			if event.Name == "" || isChmod(event) || !p.fileMatches(event.Name) {
				break
			}
			if isCreate(event) && p.cache.has(event.Name) {
				// vim "backupcopy=no" case, already collected after Rename event.
				break
			}
			if isRename(event) {
				// It is common to modify files using vim.
				// When writing to a file a backup is made. "backupcopy" option tells how it's done.
				// Default is "no": rename the file and write a new one.
				// This is cheap attempt to not send empty group for the old file.
				time.Sleep(time.Millisecond * 100)
			}
			p.refresh(ctx)
		case err := <-p.watcher.Errors:
			if err != nil {
				p.log.Warn().Err(err).Msg("watch error event")
			}
		}
	}
}

func (p *Provider) refresh(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	var added, removed []config.Config
	seen := make(map[string]bool)

	for _, file := range p.listFiles() {
		fi, err := os.Lstat(file)
		if err != nil {
			p.log.Warn().Err(err).Msgf("unable to lstat '%s'", file)
			continue
		}
		if !fi.Mode().IsRegular() {
			continue
		}

		seen[file] = true
		if v, ok := p.cache.lookup(file); ok && v.Equal(fi.ModTime()) {
			continue
		}
		p.cache.put(file, fi.ModTime())

		var cfg config.PipelineConfig
		switch err := load(&cfg, file); err {
		case nil:
			added = append(added, config.Config{Pipeline: &cfg, Source: file})
		case io.EOF:
			removed = append(removed, config.Config{Source: file})
		default:
			p.log.Warn().Err(err).Msgf("unable to load '%s'", file)
		}
	}

	for name := range p.cache {
		if seen[name] {
			continue
		}
		p.cache.remove(name)
		removed = append(removed, config.Config{Source: name})
	}

	if updates := append(added, removed...); len(updates) > 0 {
		p.send(ctx, updates)
	}
	p.watchDirs()
}

func (p *Provider) fileMatches(file string) bool {
	for _, pattern := range p.paths {
		if ok, _ := filepath.Match(pattern, file); ok {
			return true
		}
	}
	return false
}

func (p *Provider) listFiles() (files []string) {
	for _, pattern := range p.paths {
		if matches, err := filepath.Glob(pattern); err == nil {
			files = append(files, matches...)
		}
	}
	return files
}

func (p *Provider) watchDirs() {
	for _, path := range p.paths {
		if idx := strings.LastIndex(path, "/"); idx > -1 {
			path = path[:idx]
		} else {
			path = "./"
		}
		if err := p.watcher.Add(path); err != nil {
			p.log.Warn().Err(err).Msgf("unable to start watching '%s'", path)
		}
	}
}

func (p *Provider) stop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// closing the watcher deadlocks unless all events and errors are drained.
	go func() {
		for {
			select {
			case <-p.watcher.Errors:
			case <-p.watcher.Events:
			case <-ctx.Done():
				return
			}
		}
	}()

	_ = p.watcher.Close()
}

func (p *Provider) send(ctx context.Context, cfgs []config.Config) {
	if len(cfgs) == 0 {
		return
	}
	select {
	case <-ctx.Done():
	case p.configCh <- cfgs:
	}
}

func isChmod(event fsnotify.Event) bool {
	return event.Op^fsnotify.Chmod == 0
}

func isRename(event fsnotify.Event) bool {
	return event.Op&fsnotify.Rename == fsnotify.Rename
}

func isCreate(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

func load(conf interface{}, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return yaml.NewDecoder(f).Decode(conf)
}
