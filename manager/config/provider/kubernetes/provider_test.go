package kubernetes

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/netdata/sd/manager/config"
	"github.com/netdata/sd/pkg/k8s"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func TestMain(m *testing.M) {
	_ = os.Setenv(k8s.EnvFakeClient, "true")
	code := m.Run()
	_ = os.Unsetenv(k8s.EnvFakeClient)
	os.Exit(code)
}

func TestNewProvider(t *testing.T) {
	tests := map[string]struct {
		cfg       Config
		expectErr bool
	}{
		"valid config": {
			cfg: Config{ConfigMap: "cmap", Key: "key"},
		},
		"config map not set": {
			cfg:       Config{Key: "key"},
			expectErr: true,
		},
		"config map key not set": {
			cfg:       Config{ConfigMap: "cmap"},
			expectErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			p, err := NewProvider(test.cfg)

			if test.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, p)
			}
		})
	}
}

func TestProvider_Configs(t *testing.T) {
	p, err := NewProvider(Config{
		Namespace: "",
		ConfigMap: "cmap",
		Key:       "key",
	})
	require.NoError(t, err)
	assert.NotNil(t, p.Configs())
}

func TestProvider_Run(t *testing.T) {
	tests := map[string]func() runSim{
		"cmap exists before start": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap", Key: "valid.yml"}
			cmap := prepareConfigMap("cmap")
			provider, _ := prepareProvider(cfg, cmap)

			expected := []config.Config{
				cmapKeyToConfig(cmap, "valid.yml"),
			}

			sim := runSim{
				provider:        provider,
				expectedConfigs: expected,
			}
			return sim
		},
		"cmap added after start": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap", Key: "valid.yml"}
			cmap := prepareConfigMap("cmap")
			provider, client := prepareProvider(cfg)

			expected := []config.Config{
				cmapKeyToConfig(cmap, "valid.yml"),
			}

			sim := runSim{
				provider: provider,
				runAfterSync: func(ctx context.Context) {
					time.Sleep(time.Millisecond * 50)
					_, _ = client.Create(ctx, cmap, metav1.CreateOptions{})
				},
				expectedConfigs: expected,
			}
			return sim
		},
		"cmap deleted after start": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap", Key: "valid.yml"}
			cmap := prepareConfigMap("cmap")
			provider, client := prepareProvider(cfg, cmap)

			expected := []config.Config{
				cmapKeyToConfig(cmap, "valid.yml"),
				{Source: source(cmap.Namespace, cmap.Name, "valid.yml")},
			}

			sim := runSim{
				provider: provider,
				runAfterSync: func(ctx context.Context) {
					time.Sleep(time.Millisecond * 50)
					_ = client.Delete(ctx, cmap.Name, metav1.DeleteOptions{})
				},
				expectedConfigs: expected,
			}
			return sim
		},
		"cmap updated after start": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap", Key: "valid.yml"}
			cmap := prepareConfigMap("cmap")
			cmapUpdated := cmap.DeepCopy()
			cmapUpdated.Data["key"] = "value"
			provider, client := prepareProvider(cfg, cmap)

			expected := []config.Config{
				cmapKeyToConfig(cmap, "valid.yml"),
				cmapKeyToConfig(cmap, "valid.yml"),
			}

			sim := runSim{
				provider: provider,
				runAfterSync: func(ctx context.Context) {
					time.Sleep(time.Millisecond * 50)
					_, _ = client.Update(ctx, cmapUpdated, metav1.UpdateOptions{})
				},
				expectedConfigs: expected,
			}
			return sim
		},
		"several cmaps exist before run": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap1", Key: "valid.yml"}
			cmap1 := prepareConfigMap("cmap1")
			cmap2 := prepareConfigMap("cmap2")
			cmap3 := prepareConfigMap("cmap3")
			provider, _ := prepareProvider(cfg, cmap1, cmap2, cmap3)

			expected := []config.Config{
				cmapKeyToConfig(cmap1, "valid.yml"),
			}

			sim := runSim{
				provider:        provider,
				expectedConfigs: expected,
			}
			return sim
		},
		"cmap exists, but has no needed key": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap", Key: "-valid.yml"}
			cmap := prepareConfigMap("cmap")
			provider, _ := prepareProvider(cfg, cmap)

			expected := []config.Config{
				cmapKeyToConfig(cmap, "-valid.yml"),
			}

			sim := runSim{
				provider:        provider,
				expectedConfigs: expected,
			}
			return sim
		},
		"cmap exists, but key format is invalid": func() runSim {
			cfg := Config{Namespace: "default", ConfigMap: "cmap", Key: "invalid.yml"}
			cmap := prepareConfigMap("cmap")
			provider, _ := prepareProvider(cfg, cmap)

			sim := runSim{
				provider: provider,
			}
			return sim
		},
	}

	for name, sim := range tests {
		t.Run(name, func(t *testing.T) { sim().run(t) })
	}
}

func prepareConfigMap(name string) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			UID:       types.UID("a03b8dc6-dc40-46dc-b571-5030e69d8167" + name),
		},
		Data: map[string]string{
			"valid.yml":   validConfig,
			"invalid.yml": "invalid",
		},
	}
}

func prepareProvider(cfg Config, objects ...runtime.Object) (*Provider, v1.ConfigMapInterface) {
	client := fake.NewSimpleClientset(objects...)
	provider := &Provider{
		namespace: cfg.Namespace,
		cmap:      cfg.ConfigMap,
		cmapKey:   cfg.Key,
		client:    client,
		configCh:  make(chan []config.Config),
		started:   make(chan struct{}),
	}
	return provider, client.CoreV1().ConfigMaps(cfg.Namespace)
}

func cmapKeyToConfig(cmap *apiv1.ConfigMap, key string) (cfg config.Config) {
	cfg.Source = source(cmap.Namespace, cmap.Name, key)
	if data, ok := cmap.Data[key]; ok {
		_ = yaml.Unmarshal([]byte(data), &cfg.Pipeline)
	}
	return cfg
}

const validConfig = `
name: k8s
discovery:
  k8s:
    - tags: unknown
      role: pod
tag:
  - selector: unknown
    match:
      - cond: '{{ true }}'
        tags: -unknown apache
build:
  - selector: apache
    tags: file
    apply:
      - selector: apache
        tags: file
        template: |
          - module: apache
            name: apache
export:
  file:
    - selector: file
      filename: "output.conf"
`
