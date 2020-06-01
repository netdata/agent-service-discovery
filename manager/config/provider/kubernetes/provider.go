package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/netdata/sd/manager/config"
	"github.com/netdata/sd/pkg/k8s"
	"github.com/netdata/sd/pkg/log"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Config struct {
	Namespace string
	ConfigMap string
	Key       string
}

func validateConfig(cfg Config) error {
	if cfg.ConfigMap == "" {
		return errors.New("config map not set")
	}
	if cfg.Key == "" {
		return errors.New("config map key not set")
	}
	return nil
}

type Provider struct {
	namespace string
	cmap      string
	cmapKey   string
	client    kubernetes.Interface
	inf       cache.SharedInformer
	queue     *workqueue.Type
	configCh  chan []config.Config
	started   chan struct{}
	log       zerolog.Logger
}

func NewProvider(cfg Config) (*Provider, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %v", err)
	}
	p, err := initProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialization: %v", err)
	}
	return p, nil
}

func initProvider(cfg Config) (*Provider, error) {
	client, err := k8s.Clientset()
	if err != nil {
		return nil, err
	}
	p := &Provider{
		namespace: cfg.Namespace,
		cmap:      cfg.ConfigMap,
		cmapKey:   cfg.Key,
		client:    client,
		configCh:  make(chan []config.Config),
		started:   make(chan struct{}),
		queue:     workqueue.NewNamed("cmap"),
		log:       log.New("k8s config provider"),
	}
	return p, nil
}

func (p Provider) String() string {
	return source(p.namespace, p.cmap, p.cmapKey)
}

func (p *Provider) Configs() chan []config.Config {
	return p.configCh
}

func (p *Provider) Run(ctx context.Context) {
	p.log.Info().Msg("instance is started")
	defer p.log.Info().Msg("instance is stopped")
	defer p.queue.ShutDown()

	p.inf = p.setupInformer(ctx)
	go p.inf.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), p.inf.HasSynced) {
		p.log.Error().Msg("unable to sync caches")
		return
	}

	go p.run(ctx)
	close(p.started)

	<-ctx.Done()
}

const resyncPeriod = 10 * time.Minute

func (p *Provider) setupInformer(ctx context.Context) cache.SharedInformer {
	client := p.client.CoreV1().ConfigMaps(p.namespace)
	clw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.Watch(ctx, options)
		},
	}
	inf := cache.NewSharedInformer(clw, &apiv1.ConfigMap{}, resyncPeriod)
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { p.enqueue(obj) },
		UpdateFunc: func(_, obj interface{}) { p.enqueue(obj) },
		DeleteFunc: func(obj interface{}) { p.enqueue(obj) },
	})
	return inf
}
func (p *Provider) enqueue(obj interface{}) {
	cmap, err := toConfigMap(obj)
	if err != nil || p.cmap != cmap.Name {
		return
	}
	if p.namespace != apiv1.NamespaceAll && p.namespace != cmap.Namespace {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	p.queue.Add(key)
}

func (p *Provider) run(ctx context.Context) {
	for {
		item, shutdown := p.queue.Get()
		if shutdown {
			return
		}

		func() {
			defer p.queue.Done(item)

			key := item.(string)
			namespace, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return
			}

			item, exists, err := p.inf.GetStore().GetByKey(key)
			if err != nil {
				return
			}
			cfg := config.Config{Source: source(namespace, name, p.cmapKey)}

			if !exists {
				p.send(ctx, cfg)
				return
			}

			cmap, err := toConfigMap(item)
			if err != nil {
				return
			}

			data, ok := cmap.Data[p.cmapKey]
			if !ok {
				p.log.Debug().Msgf("cmap '%s/%s' has no '%s' key", cmap.Namespace, cmap.Name, p.cmapKey)
				p.send(ctx, cfg)
				return
			}

			if err := yaml.Unmarshal([]byte(data), &cfg.Pipeline); err != nil {
				p.log.Error().Err(err).Msgf("decode cmap '%s/%s' key '%s'", cmap.Namespace, cmap.Name, p.cmapKey)
				return
			}
			p.send(ctx, cfg)
		}()
	}
}

func (p *Provider) send(ctx context.Context, cfg config.Config) {
	select {
	case <-ctx.Done():
	case p.configCh <- []config.Config{cfg}:
	}
}

func source(ns, name, key string) string {
	return fmt.Sprintf("k8s/cmap/%s/%s:%s", ns, name, key)
}

func toConfigMap(item interface{}) (*apiv1.ConfigMap, error) {
	cmap, ok := item.(*apiv1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("received unexpected object type: %T", item)
	}
	return cmap, nil
}
