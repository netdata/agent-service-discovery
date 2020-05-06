package kubernetes

import (
	"errors"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func clientSet() (*kubernetes.Clientset, error) {
	if isInCluster() {
		return clientSetInCluster()
	}
	return clientSetOutOfCluster()
}

func clientSetInCluster() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	config.UserAgent = "Netdata/autodiscovery"
	return kubernetes.NewForConfig(config)
}

func clientSetOutOfCluster() (*kubernetes.Clientset, error) {
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

func isInCluster() bool {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	return len(host) > 0 && len(port) > 0
}
