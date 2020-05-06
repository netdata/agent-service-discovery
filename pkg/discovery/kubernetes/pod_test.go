package kubernetes

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/netdata/sd/pkg/model"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestPodGroup_Source(t *testing.T) {
	tests := map[string]struct {
		createSim      func() discoverySimTest
		expectedSource []string
	}{
		"pods with multiple ports": {
			createSim: func() discoverySimTest {
				httpd, nginx := newHTTPDPod(), newNGINXPod()
				discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

				sim := discoverySimTest{
					discovery: discovery,
					expectedGroups: []model.Group{
						preparePodGroup(httpd),
						preparePodGroup(nginx),
					},
				}
				return sim
			},
			expectedSource: []string{
				"k8s/pod/default/httpd-dd95c4d68-5bkwl",
				"k8s/pod/default/nginx-7cfd77469b-q6kxj",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sim := test.createSim()
			var actual []string
			for _, group := range sim.run(t) {
				actual = append(actual, group.Source())
			}

			assert.Equal(t, test.expectedSource, actual)
		})
	}
}

func TestPodGroup_Targets(t *testing.T) {
	tests := map[string]struct {
		createSim          func() discoverySimTest
		expectedNumTargets int
	}{
		"pods with multiple ports": {
			createSim: func() discoverySimTest {
				httpd, nginx := newHTTPDPod(), newNGINXPod()
				discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

				sim := discoverySimTest{
					discovery: discovery,
					expectedGroups: []model.Group{
						preparePodGroup(httpd),
						preparePodGroup(nginx),
					},
				}
				return sim
			},
			expectedNumTargets: 4,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sim := test.createSim()
			var actual int
			for _, group := range sim.run(t) {
				actual += len(group.Targets())
			}

			assert.Equal(t, test.expectedNumTargets, actual)
		})
	}
}

func TestPodTarget_Hash(t *testing.T) {
	tests := map[string]struct {
		createSim    func() discoverySimTest
		expectedHash []uint64
	}{
		"pods with multiple ports": {
			createSim: func() discoverySimTest {
				httpd, nginx := newHTTPDPod(), newNGINXPod()
				discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

				sim := discoverySimTest{
					discovery: discovery,
					expectedGroups: []model.Group{
						preparePodGroup(httpd),
						preparePodGroup(nginx),
					},
				}
				return sim
			},
			expectedHash: []uint64{
				9290392315829965134,
				5860818937031469805,
				4811247736538096710,
				5217770314243214482,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sim := test.createSim()
			var actual []uint64
			for _, group := range sim.run(t) {
				for _, tg := range group.Targets() {
					actual = append(actual, tg.Hash())
				}
			}

			assert.Equal(t, test.expectedHash, actual)
		})
	}
}

func TestPodTarget_TUID(t *testing.T) {
	tests := map[string]struct {
		createSim    func() discoverySimTest
		expectedTUID []string
	}{
		"pods with multiple ports": {
			createSim: func() discoverySimTest {
				httpd, nginx := newHTTPDPod(), newNGINXPod()
				discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

				sim := discoverySimTest{
					discovery: discovery,
					expectedGroups: []model.Group{
						preparePodGroup(httpd),
						preparePodGroup(nginx),
					},
				}
				return sim
			},
			expectedTUID: []string{
				"default_httpd-dd95c4d68-5bkwl_httpd_tcp_80",
				"default_httpd-dd95c4d68-5bkwl_httpd_tcp_443",
				"default_nginx-7cfd77469b-q6kxj_nginx_tcp_80",
				"default_nginx-7cfd77469b-q6kxj_nginx_tcp_443",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sim := test.createSim()
			var actual []string
			for _, group := range sim.run(t) {
				for _, tg := range group.Targets() {
					actual = append(actual, tg.TUID())
				}
			}

			assert.Equal(t, test.expectedTUID, actual)
		})
	}
}

func TestNewPod(t *testing.T) {
	tests := map[string]struct {
		informer  cache.SharedInformer
		wantPanic bool
	}{
		"valid informer": {informer: cache.NewSharedInformer(nil, &apiv1.Pod{}, resyncPeriod)},
		"nil informer":   {wantPanic: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.wantPanic {
				assert.Panics(t, func() { NewPod(nil) })
			} else {
				assert.IsType(t, &Pod{}, NewPod(test.informer))
			}
		})
	}
}

func TestPod_String(t *testing.T) {
	assert.NotEmpty(t, Pod{}.String())
}

func TestPod_Discover(t *testing.T) {
	tests := map[string]func() discoverySimTest{
		"ADD: pods exist before run": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

			sim := discoverySimTest{
				discovery: discovery,
				expectedGroups: []model.Group{
					preparePodGroup(httpd),
					preparePodGroup(nginx),
				},
			}
			return sim
		},
		"ADD: pods exist before run and add after sync": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			discovery, clientset := prepareAllNsDiscovery(RolePod, httpd)
			podClient := clientset.CoreV1().Pods("default")

			sim := discoverySimTest{
				discovery: discovery,
				runAfterSync: func(ctx context.Context) {
					_, _ = podClient.Create(ctx, nginx, metav1.CreateOptions{})
				},
				expectedGroups: []model.Group{
					preparePodGroup(httpd),
					preparePodGroup(nginx),
				},
			}
			return sim
		},
		"DELETE: remove pods after sync": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			discovery, clientset := prepareAllNsDiscovery(RolePod, httpd, nginx)
			podClient := clientset.CoreV1().Pods("default")

			sim := discoverySimTest{
				discovery: discovery,
				runAfterSync: func(ctx context.Context) {
					time.Sleep(time.Millisecond * 50)
					_ = podClient.Delete(ctx, httpd.Name, metav1.DeleteOptions{})
					_ = podClient.Delete(ctx, nginx.Name, metav1.DeleteOptions{})
				},
				expectedGroups: []model.Group{
					preparePodGroup(httpd),
					preparePodGroup(nginx),
					prepareEmptyPodGroup(httpd),
					prepareEmptyPodGroup(nginx),
				},
			}
			return sim
		},
		"DELETE,ADD: remove and add pods after sync": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			discovery, clientset := prepareAllNsDiscovery(RolePod, httpd)
			podClient := clientset.CoreV1().Pods("default")

			sim := discoverySimTest{
				discovery: discovery,
				runAfterSync: func(ctx context.Context) {
					time.Sleep(time.Millisecond * 50)
					_ = podClient.Delete(ctx, httpd.Name, metav1.DeleteOptions{})
					_, _ = podClient.Create(ctx, nginx, metav1.CreateOptions{})
				},
				expectedGroups: []model.Group{
					preparePodGroup(httpd),
					prepareEmptyPodGroup(httpd),
					preparePodGroup(nginx),
				},
			}
			return sim
		},
		"ADD: pods with empty PodIP": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			httpd.Status.PodIP = ""
			nginx.Status.PodIP = ""
			discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

			sim := discoverySimTest{
				discovery: discovery,
				expectedGroups: []model.Group{
					prepareEmptyPodGroup(httpd),
					prepareEmptyPodGroup(nginx),
				},
			}
			return sim
		},
		"UPDATE: set pods PodIP after sync": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			httpd.Status.PodIP = ""
			nginx.Status.PodIP = ""
			discovery, clientset := prepareAllNsDiscovery(RolePod, httpd, nginx)
			podClient := clientset.CoreV1().Pods("default")

			sim := discoverySimTest{
				discovery: discovery,
				runAfterSync: func(ctx context.Context) {
					time.Sleep(time.Millisecond * 50)
					_, _ = podClient.Update(ctx, newHTTPDPod(), metav1.UpdateOptions{})
					_, _ = podClient.Update(ctx, newNGINXPod(), metav1.UpdateOptions{})
				},
				expectedGroups: []model.Group{
					prepareEmptyPodGroup(httpd),
					prepareEmptyPodGroup(nginx),
					preparePodGroup(newHTTPDPod()),
					preparePodGroup(newNGINXPod()),
				},
			}
			return sim
		},
		"ADD: pods without containers": func() discoverySimTest {
			httpd, nginx := newHTTPDPod(), newNGINXPod()
			httpd.Spec.Containers = httpd.Spec.Containers[:0]
			nginx.Spec.Containers = httpd.Spec.Containers[:0]
			discovery, _ := prepareAllNsDiscovery(RolePod, httpd, nginx)

			sim := discoverySimTest{
				discovery: discovery,
				expectedGroups: []model.Group{
					prepareEmptyPodGroup(httpd),
					prepareEmptyPodGroup(nginx),
				},
			}
			return sim
		},
	}

	for name, createSim := range tests {
		t.Run(name, func(t *testing.T) { createSim().run(t) })
	}
}

func newHTTPDPod() *apiv1.Pod {
	return &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "httpd-dd95c4d68-5bkwl",
			Namespace:   "default",
			UID:         "1cebb6eb-0c1e-495b-8131-8fa3e6668dc8",
			Annotations: map[string]string{"phase": "prod"},
			Labels:      map[string]string{"app": "httpd", "tier": "frontend"},
		},
		Spec: apiv1.PodSpec{
			NodeName: "m01",
			Containers: []apiv1.Container{
				{
					Name:  "httpd",
					Image: "httpd",
					Env: []apiv1.EnvVar{
						{Name: "", Value: ""},
						{Name: "", Value: ""},
					},
					Ports: []apiv1.ContainerPort{
						{Name: "http", Protocol: apiv1.ProtocolTCP, ContainerPort: 80},
						{Name: "https", Protocol: apiv1.ProtocolTCP, ContainerPort: 443},
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP: "172.17.0.1",
		},
	}
}

func newNGINXPod() *apiv1.Pod {
	return &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nginx-7cfd77469b-q6kxj",
			Namespace:   "default",
			UID:         "09e883f2-d740-4c5f-970d-02cf02876522",
			Annotations: map[string]string{"phase": "prod"},
			Labels:      map[string]string{"app": "nginx", "tier": "frontend"},
		},
		Spec: apiv1.PodSpec{
			NodeName: "m01",
			Containers: []apiv1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
					Env: []apiv1.EnvVar{
						{Name: "", Value: ""},
						{Name: "", Value: ""},
					},
					Ports: []apiv1.ContainerPort{
						{Name: "http", Protocol: apiv1.ProtocolTCP, ContainerPort: 80},
						{Name: "https", Protocol: apiv1.ProtocolTCP, ContainerPort: 443},
					},
				},
			},
		},
		Status: apiv1.PodStatus{
			PodIP: "172.17.0.2",
		},
	}
}

func prepareEmptyPodGroup(pod *apiv1.Pod) *podGroup {
	return &podGroup{source: podSource(pod)}
}

func preparePodGroup(pod *apiv1.Pod) *podGroup {
	group := prepareEmptyPodGroup(pod)
	for _, container := range pod.Spec.Containers {
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
				Env:          nil,
				PortNumber:   portNum,
				PortName:     port.Name,
				PortProtocol: string(port.Protocol),
			}
			target.hash = mustCalcHash(target)
			target.Tags().Merge(discoveryTags)
			group.targets = append(group.targets, target)
		}
	}
	return group
}
