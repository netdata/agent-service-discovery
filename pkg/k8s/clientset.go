package k8s

import (
	"errors"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	EnvFakeClient = "KUBERNETES_FAKE_CLIENTSET"
)

func Clientset() (kubernetes.Interface, error) {
	switch {
	case os.Getenv(EnvFakeClient) != "":
		return fake.NewSimpleClientset(), nil
	case InCluster():
		return clientsetInCluster()
	default:
		return clientsetOutOfCluster()
	}
}

func clientsetInCluster() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	config.UserAgent = "Netdata/auto-discovery"
	return kubernetes.NewForConfig(config)
}

func clientsetOutOfCluster() (*kubernetes.Clientset, error) {
	home := homeDir()
	if home == "" {
		return nil, errors.New("couldn't find home directory")
	}
	configPath := filepath.Join(home, ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, err
	}
	config.UserAgent = "Netdata/auto-discovery"
	return kubernetes.NewForConfig(config)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func InCluster() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != "" && os.Getenv("KUBERNETES_SERVICE_PORT") != ""
}
