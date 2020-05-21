package kubernetes

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/netdata/sd/pkg/model"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type (
	podGroup struct {
		targets []model.Target
		source  string
	}
	PodTarget struct {
		model.Base `hash:"ignore"`
		hash       uint64
		tuid       string
		Address    string

		Namespace   string
		Name        string
		Annotations map[string]string
		Labels      map[string]string
		NodeName    string
		PodIP       string

		ContName     string
		Image        string
		Env          map[string]string
		PortNumber   string
		PortName     string
		PortProtocol string
	}
)

func (pt PodTarget) Hash() uint64 { return pt.hash }
func (pt PodTarget) TUID() string { return pt.tuid }

func (pg podGroup) Source() string          { return pg.source }
func (pg podGroup) Targets() []model.Target { return pg.targets }

type Pod struct {
	podInformer    cache.SharedInformer
	cmapInformer   cache.SharedInformer
	secretInformer cache.SharedInformer
	queue          *workqueue.Type
}

func NewPod(pod, cfgMap, secret cache.SharedInformer) *Pod {
	queue := workqueue.NewNamed("pod")
	pod.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { enqueue(queue, obj) },
		UpdateFunc: func(_, obj interface{}) { enqueue(queue, obj) },
		DeleteFunc: func(obj interface{}) { enqueue(queue, obj) },
	})

	return &Pod{
		podInformer:    pod,
		cmapInformer:   cfgMap,
		secretInformer: secret,
		queue:          queue,
	}
}

func (p Pod) String() string {
	return fmt.Sprintf("k8s role: %s", RolePod)
}

func (p *Pod) Discover(ctx context.Context, in chan<- []model.Group) {
	defer p.queue.ShutDown()

	go p.podInformer.Run(ctx.Done())
	go p.cmapInformer.Run(ctx.Done())
	go p.secretInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(),
		p.podInformer.HasSynced, p.cmapInformer.HasSynced, p.secretInformer.HasSynced) {
		return
	}
	go func() {
		for p.processOnce(ctx, in) {
		}
	}()
	<-ctx.Done()
}

func (p *Pod) processOnce(ctx context.Context, in chan<- []model.Group) bool {
	item, shutdown := p.queue.Get()
	if shutdown {
		return false
	}
	defer p.queue.Done(item)

	key := item.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return true
	}
	item, exists, err := p.podInformer.GetStore().GetByKey(key)
	if err != nil {
		return true
	}
	if !exists {
		send(ctx, in, &podGroup{source: podSourceFromNsName(namespace, name)})
		return true
	}
	pod, err := covertToPod(item)
	if err != nil {
		return true
	}
	send(ctx, in, p.buildGroup(pod))
	return true
}

func (p Pod) buildGroup(pod *apiv1.Pod) model.Group {
	if pod.Status.PodIP == "" || len(pod.Spec.Containers) == 0 {
		return &podGroup{
			source: podSource(pod),
		}
	}
	return &podGroup{
		source:  podSource(pod),
		targets: p.buildTargets(pod),
	}
}

func (p Pod) buildTargets(pod *apiv1.Pod) (targets []model.Target) {
	for _, container := range pod.Spec.Containers {
		env := p.collectENV(pod.Namespace, container)

		for _, port := range container.Ports {
			portNum := strconv.FormatUint(uint64(port.ContainerPort), 10)
			target := &PodTarget{
				tuid:         podTUID(pod, container, port),
				Address:      net.JoinHostPort(pod.Status.PodIP, portNum),
				Namespace:    pod.Namespace,
				Name:         pod.Name,
				Annotations:  pod.Annotations,
				Labels:       pod.Labels,
				NodeName:     pod.Spec.NodeName,
				PodIP:        pod.Status.PodIP,
				ContName:     container.Name,
				Image:        container.Image,
				Env:          env,
				PortNumber:   portNum,
				PortName:     port.Name,
				PortProtocol: string(port.Protocol),
			}
			hash, err := calcHash(target)
			if err != nil {
				continue
			}
			target.hash = hash

			targets = append(targets, target)
		}
	}
	return targets
}

func (p Pod) collectENV(namespace string, container apiv1.Container) map[string]string {
	_ = container.Env
	_ = container.EnvFrom
	return nil
}

func podTUID(pod *apiv1.Pod, container apiv1.Container, port apiv1.ContainerPort) string {
	return fmt.Sprintf("%s_%s_%s_%s_%s",
		pod.Namespace,
		pod.Name,
		container.Name,
		strings.ToLower(string(port.Protocol)),
		strconv.FormatUint(uint64(port.ContainerPort), 10),
	)
}

func podSourceFromNsName(namespace, name string) string {
	return "k8s/pod/" + namespace + "/" + name
}

func podSource(pod *apiv1.Pod) string {
	return podSourceFromNsName(pod.Namespace, pod.Name)
}

func covertToPod(item interface{}) (*apiv1.Pod, error) {
	pod, ok := item.(*apiv1.Pod)
	if !ok {
		return nil, fmt.Errorf("received unexpected object type: %T", item)
	}
	return pod, nil
}

func isVariable(s string) bool {
	// Variable references $(VAR_NAME) are expanded using the previous defined
	// environment variables in the container and any service environment
	// variables.
	return strings.IndexByte(s, '$') != -1
}
