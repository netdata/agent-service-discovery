package kubernetes

import (
	"context"
	"encoding/base64"
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

func NewPod(pod, cmap, secret cache.SharedInformer) *Pod {
	queue := workqueue.NewNamed("pod")
	pod.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { enqueue(queue, obj) },
		UpdateFunc: func(_, obj interface{}) { enqueue(queue, obj) },
		DeleteFunc: func(obj interface{}) { enqueue(queue, obj) },
	})

	return &Pod{
		podInformer:    pod,
		cmapInformer:   cmap,
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
	pod, err := convertToPod(item)
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
		env := p.collectEnv(pod.Namespace, container)

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

func (p Pod) collectEnv(ns string, container apiv1.Container) map[string]string {
	vars := make(map[string]string)

	// When a key exists in multiple sources,
	// the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	for _, source := range container.EnvFrom {
		p.envFromConfigMap(vars, ns, source)
		p.envFromSecret(vars, ns, source)
	}

	for _, env := range container.Env {
		if env.Name == "" || isVar(env.Name) {
			continue
		}
		if env.Value != "" {
			vars[env.Name] = env.Value
		} else {
			p.valueFromConfigMap(vars, ns, env)
			p.valueFromSecret(vars, ns, env)
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return vars
}

func (p Pod) valueFromConfigMap(vars map[string]string, ns string, env apiv1.EnvVar) {
	switch {
	case
		env.ValueFrom == nil,
		env.ValueFrom.ConfigMapKeyRef == nil,
		env.ValueFrom.ConfigMapKeyRef.Name == "",
		env.ValueFrom.ConfigMapKeyRef.Key == "":
		return
	}

	sr := env.ValueFrom.ConfigMapKeyRef
	key := ns + "/" + sr.Name
	item, exist, err := p.cmapInformer.GetStore().GetByKey(key)
	if err != nil || !exist {
		return
	}
	cmap, err := convertToConfigMap(item)
	if err != nil {
		return
	}
	if v, ok := cmap.Data[sr.Key]; ok {
		vars[env.Name] = v
	}
}

func (p Pod) valueFromSecret(vars map[string]string, ns string, env apiv1.EnvVar) {
	switch {
	case
		env.ValueFrom == nil,
		env.ValueFrom.SecretKeyRef == nil,
		env.ValueFrom.SecretKeyRef.Name == "",
		env.ValueFrom.SecretKeyRef.Key == "":
		return
	}

	sr := env.ValueFrom.SecretKeyRef
	key := ns + "/" + sr.Name
	item, exist, err := p.secretInformer.GetStore().GetByKey(key)
	if err != nil || !exist {
		return
	}
	secret, err := convertToSecret(item)
	if err != nil {
		return
	}
	if v, ok := secret.Data[sr.Key]; ok {
		vars[env.Name] = decode64(v)
	}
}

func (p Pod) envFromConfigMap(vars map[string]string, ns string, src apiv1.EnvFromSource) {
	if src.ConfigMapRef == nil || src.ConfigMapRef.Name == "" {
		return
	}
	key := ns + "/" + src.ConfigMapRef.Name
	item, exist, err := p.cmapInformer.GetStore().GetByKey(key)
	if err != nil || !exist {
		return
	}
	cmap, err := convertToConfigMap(item)
	if err != nil {
		return
	}
	for k, v := range cmap.Data {
		vars[src.Prefix+k] = v
	}
}

func (p Pod) envFromSecret(vars map[string]string, ns string, src apiv1.EnvFromSource) {
	if src.SecretRef == nil || src.SecretRef.Name == "" {
		return
	}
	key := ns + "/" + src.SecretRef.Name
	item, exist, err := p.secretInformer.GetStore().GetByKey(key)
	if err != nil || !exist {
		return
	}
	secret, err := convertToSecret(item)
	if err != nil {
		return
	}
	for k, v := range secret.Data {
		vars[src.Prefix+k] = decode64(v)
	}
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

func convertToPod(item interface{}) (*apiv1.Pod, error) {
	pod, ok := item.(*apiv1.Pod)
	if !ok {
		return nil, fmt.Errorf("received unexpected object type: %T", item)
	}
	return pod, nil
}

func convertToConfigMap(item interface{}) (*apiv1.ConfigMap, error) {
	cmap, ok := item.(*apiv1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("received unexpected object type: %T", item)
	}
	return cmap, nil
}

func convertToSecret(item interface{}) (*apiv1.Secret, error) {
	secret, ok := item.(*apiv1.Secret)
	if !ok {
		return nil, fmt.Errorf("received unexpected object type: %T", item)
	}
	return secret, nil
}

func isVar(s string) bool {
	// Variable references $(VAR_NAME) are expanded using the previous defined
	// environment variables in the container and any service environment
	// variables.
	return strings.IndexByte(s, '$') != -1
}

func decode64(bs []byte) string {
	if len(bs) == 0 {
		return ""
	}
	bs, _ = base64.StdEncoding.DecodeString(string(bs))
	return string(bs)
}
