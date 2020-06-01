package export

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/netdata/sd/pipeline/model"

	"github.com/stretchr/testify/assert"
)

func TestManager_Export(t *testing.T) {
	e1, e2, e3 := &mockExporter{}, &mockExporter{}, &mockExporter{}
	mgr := &Manager{exporters: []exporter{e1, e2, e3}}
	out := make(chan []model.Config)
	wantCfgs := []model.Config{{Conf: "1"}, {Conf: "2"}, {Conf: "2"}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wg.Done()
		mgr.Export(ctx, out)
	}()

	const timeout = time.Second * 2
	tk := time.NewTicker(timeout)
	defer tk.Stop()

	select {
	case out <- wantCfgs:
	case <-tk.C:
		t.Errorf("exporting timed out in %s", timeout)
		return
	}

	time.Sleep(time.Second)
	cancel()
	wg.Wait()

	assert.Equal(t, wantCfgs, e1.seen)
	assert.Equal(t, wantCfgs, e2.seen)
	assert.Equal(t, wantCfgs, e3.seen)
}

type mockExporter struct {
	seen []model.Config
}

func (e *mockExporter) Export(ctx context.Context, out <-chan []model.Config) {
	select {
	case <-ctx.Done():
	case cfgs := <-out:
		e.seen = append(e.seen, cfgs...)
	}
	<-ctx.Done()
}
