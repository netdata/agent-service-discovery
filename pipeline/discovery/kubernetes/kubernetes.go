package kubernetes

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/netdata/sd/pipeline/model"
	"github.com/netdata/sd/pkg/k8s"
	"github.com/netdata/sd/pkg/log"

	"github.com/ilyam8/hashstructure"
	"github.com/rs/zerolog"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Role string

const (
	RolePod     = "pod"
	RoleService = "service"
)

func isRoleValid(role string) bool { return role == RolePod || role == RoleService }

const (
	envNodeName = "MY_NODE_NAME"
)

type Config struct {
	APIServer  string   `yaml:"api_server"`
	Tags       string   `yaml:"tags"`
	Namespaces []string `yaml:"namespaces"`
	Role       string   `yaml:"role"`
	LocalMode  bool     `yaml:"local_mode"`
	Selector   struct {
		Label string `yaml:"label"`
		Field string `yaml:"field"`
	} `yaml:"selector"`
}

func validateConfig(cfg Config) error {
	if !isRoleValid(cfg.Role) {
		return fmt.Errorf("invalid role '%s', valid roles: '%s', '%s'", cfg.Role, RolePod, RoleService)
	}
	if cfg.Tags == "" {
		return fmt.Errorf("no tags set for '%s' role", cfg.Role)
	}
	return nil
}

type (
	discoverer interface {
		Discover(ctx context.Context, ch chan<- []model.Group)
	}
	Discovery struct {
		tags          model.Tags
		namespaces    []string
		role          string
		selectorLabel string
		selectorField string
		client        kubernetes.Interface
		discoverers   []discoverer
		started       chan struct{}
		log           zerolog.Logger
	}
)

func NewDiscovery(cfg Config) (*Discovery, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("k8s discovery config validation: %v", err)
	}

	d, err := initDiscovery(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s discovery initialization ('%s'): %v", cfg.Role, err)
	}
	return d, nil
}

func initDiscovery(cfg Config) (*Discovery, error) {
	tags, err := model.ParseTags(cfg.Tags)
	if err != nil {
		return nil, fmt.Errorf("parse config->tags: %v", err)
	}
	client, err := k8s.Clientset()
	if err != nil {
		return nil, fmt.Errorf("create clientset: %v", err)
	}
	namespaces := cfg.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{apiv1.NamespaceAll}
	}
	if cfg.LocalMode && cfg.Role == RolePod {
		if name := os.Getenv(envNodeName); name != "" {
			cfg.Selector.Field = joinSelectors(cfg.Selector.Field, "spec.nodeName="+name)
		} else {
			return nil, fmt.Errorf("local_mode is enabled, but env '%s' not set", envNodeName)
		}
	}

	d := &Discovery{
		tags:          tags,
		namespaces:    namespaces,
		role:          cfg.Role,
		selectorLabel: cfg.Selector.Label,
		selectorField: cfg.Selector.Field,
		client:        client,
		discoverers:   make([]discoverer, 0, len(namespaces)),
		started:       make(chan struct{}),
		log:           log.New("k8s discovery manager"),
	}
	return d, nil
}

func (d *Discovery) String() string {
	return "k8s discovery manager"
}

const resyncPeriod = 10 * time.Minute

func (d *Discovery) Discover(ctx context.Context, in chan<- []model.Group) {
	for _, namespace := range d.namespaces {
		var dd discoverer
		switch d.role {
		case RolePod:
			dd = d.setupPodDiscoverer(ctx, namespace)
		case RoleService:
			dd = d.setupServiceDiscoverer(ctx, namespace)
		default:
			panic(fmt.Sprintf("unknown k8 discovery role: '%s'", d.role))
		}
		d.discoverers = append(d.discoverers, dd)
	}
	if len(d.discoverers) == 0 {
		panic("k8s cant run discovery: zero discoverers")
	}

	d.log.Info().Msgf("registered: %v", d.discoverers)

	var wg sync.WaitGroup
	updates := make(chan []model.Group)

	for _, dd := range d.discoverers {
		wg.Add(1)
		go func(dd discoverer) { defer wg.Done(); dd.Discover(ctx, updates) }(dd)
	}

	wg.Add(1)
	go func() { defer wg.Done(); d.run(ctx, updates, in) }()

	close(d.started)

	wg.Wait()
	<-ctx.Done()
}

func (d *Discovery) run(ctx context.Context, updates chan []model.Group, in chan<- []model.Group) {
	for {
		select {
		case <-ctx.Done():
			return
		case groups := <-updates:
			for _, group := range groups {
				for _, t := range group.Targets() {
					t.Tags().Merge(d.tags)
				}
			}
			select {
			case <-ctx.Done():
				return
			case in <- groups:
			}
		}
	}
}

func (d *Discovery) setupPodDiscoverer(ctx context.Context, namespace string) *Pod {
	pod := d.client.CoreV1().Pods(namespace)
	podLW := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = d.selectorField
			options.LabelSelector = d.selectorLabel
			return pod.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = d.selectorField
			options.LabelSelector = d.selectorLabel
			return pod.Watch(ctx, options)
		},
	}

	cmap := d.client.CoreV1().ConfigMaps(namespace)
	cmapLW := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return cmap.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return cmap.Watch(ctx, options)
		},
	}

	secret := d.client.CoreV1().Secrets(namespace)
	secretLW := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return secret.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return secret.Watch(ctx, options)
		},
	}

	return NewPod(
		cache.NewSharedInformer(podLW, &apiv1.Pod{}, resyncPeriod),
		cache.NewSharedInformer(cmapLW, &apiv1.ConfigMap{}, resyncPeriod),
		cache.NewSharedInformer(secretLW, &apiv1.Secret{}, resyncPeriod),
	)
}

func (d *Discovery) setupServiceDiscoverer(ctx context.Context, namespace string) *Service {
	svc := d.client.CoreV1().Services(namespace)
	clw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = d.selectorField
			options.LabelSelector = d.selectorLabel
			return svc.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = d.selectorField
			options.LabelSelector = d.selectorLabel
			return svc.Watch(ctx, options)
		},
	}
	inf := cache.NewSharedInformer(clw, &apiv1.Service{}, resyncPeriod)
	return NewService(inf)
}

func enqueue(queue *workqueue.Type, obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	queue.Add(key)
}

func send(ctx context.Context, in chan<- []model.Group, group model.Group) {
	if group == nil {
		return
	}
	select {
	case <-ctx.Done():
	case in <- []model.Group{group}:
	}
}

func calcHash(obj interface{}) (uint64, error) {
	return hashstructure.Hash(obj, nil)
}

func joinSelectors(srs ...string) string {
	var i int
	for _, v := range srs {
		if v != "" {
			srs[i] = v
			i++
		}
	}
	return strings.Join(srs[:i], ",")
}
