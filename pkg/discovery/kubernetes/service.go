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
	serviceGroup struct {
		targets []model.Target
		source  string
	}
	ServiceTarget struct {
		model.Base `hash:"ignore"`
		hash       uint64
		tuid       string
		Address    string

		Namespace    string
		Name         string
		Annotations  map[string]string
		Labels       map[string]string
		PortNumber   string
		PortName     string
		PortProtocol string
		ClusterIP    string
		ExternalName string
		Type         string
	}
)

func (st ServiceTarget) Hash() uint64 { return st.hash }
func (st ServiceTarget) TUID() string { return st.tuid }

func (sg serviceGroup) Source() string          { return sg.source }
func (sg serviceGroup) Targets() []model.Target { return sg.targets }

type Service struct {
	informer cache.SharedInformer
	queue    *workqueue.Type
}

func NewService(inf cache.SharedInformer) *Service {
	queue := workqueue.NewNamed("service")
	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { enqueue(queue, obj) },
		UpdateFunc: func(_, obj interface{}) { enqueue(queue, obj) },
		DeleteFunc: func(obj interface{}) { enqueue(queue, obj) },
	})

	return &Service{
		informer: inf,
		queue:    queue,
	}
}

func (s Service) String() string {
	return fmt.Sprintf("k8s role: %s", RoleService)
}

func (s *Service) Discover(ctx context.Context, ch chan<- []model.Group) {
	defer s.queue.ShutDown()
	go s.informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), s.informer.HasSynced) {
		return
	}
	go func() {
		for s.processOnce(ctx, ch) {
		}
	}()
	<-ctx.Done()
}

func (s *Service) processOnce(ctx context.Context, ch chan<- []model.Group) bool {
	item, shutdown := s.queue.Get()
	if shutdown {
		return false
	}
	defer s.queue.Done(item)

	key := item.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return true
	}
	item, exists, err := s.informer.GetStore().GetByKey(key)
	if err != nil {
		return true
	}
	if !exists {
		send(ctx, ch, &serviceGroup{source: serviceSourceFromNsName(namespace, name)})
		return true
	}
	svc, err := toService(item)
	if err != nil {
		return true
	}
	send(ctx, ch, s.buildGroup(svc))
	return true
}

func (s Service) buildGroup(svc *apiv1.Service) model.Group {
	// TODO: headless service?
	if svc.Spec.ClusterIP == "" || len(svc.Spec.Ports) == 0 {
		return &serviceGroup{
			source: serviceSource(svc),
		}
	}
	return &serviceGroup{
		source:  serviceSource(svc),
		targets: s.buildTargets(svc),
	}
}

func (s Service) buildTargets(svc *apiv1.Service) (targets []model.Target) {
	for _, port := range svc.Spec.Ports {
		portNum := strconv.FormatInt(int64(port.Port), 10)
		target := &ServiceTarget{
			tuid:         serviceTUID(svc, port),
			Address:      net.JoinHostPort(svc.Name+"."+svc.Namespace+".svc", portNum),
			Namespace:    svc.Namespace,
			Name:         svc.Name,
			Annotations:  svc.Annotations,
			Labels:       svc.Labels,
			PortNumber:   portNum,
			PortName:     port.Name,
			PortProtocol: string(port.Protocol),
			ClusterIP:    svc.Spec.ClusterIP,
			ExternalName: svc.Spec.ExternalName,
			Type:         string(svc.Spec.Type),
		}
		hash, err := calcHash(target)
		if err != nil {
			continue
		}
		target.hash = hash

		targets = append(targets, target)
	}
	return targets
}

func serviceTUID(svc *apiv1.Service, port apiv1.ServicePort) string {
	return fmt.Sprintf("%s_%s_%s_%s",
		svc.Namespace,
		svc.Name,
		strings.ToLower(string(port.Protocol)),
		strconv.FormatInt(int64(port.Port), 10),
	)
}

func serviceSourceFromNsName(namespace, name string) string {
	return "k8s/service/" + namespace + "/" + name
}

func serviceSource(svc *apiv1.Service) string {
	return serviceSourceFromNsName(svc.Namespace, svc.Name)
}

func toService(o interface{}) (*apiv1.Service, error) {
	svc, ok := o.(*apiv1.Service)
	if !ok {
		return nil, fmt.Errorf("received unexpected object type: %T", o)
	}
	return svc, nil
}
