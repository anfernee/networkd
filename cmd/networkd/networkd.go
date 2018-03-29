package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/golang/glog"
)

const (
	cniPath           = "/host/etc/cni/net.d/10-ptp.conflist"
	cniConfigTemplate = `
	{
  		"name": "gce-pod-network",
  		"cniVersion": "0.3.1",
  		"plugins": [
    		{
      			"type": "ptp",
      			"mtu": 1460,
      			"ipam": {
        			"type": "host-local",
        			"subnet": "%s",
					"routes": [
	  					{"dst": "0.0.0.0/0"}
	  				]
	  			}
    		}
  		]
	}`
)

func main() {
	nodeName, err := os.Hostname()
	if err != nil {
		glog.Error("failed to get node name: %v", err)
		return
	}

	client, err := k8sClient()
	if err != nil {
		return
	}

	var cidr string

	for {
		_, err := os.Stat(cniPath)
		if err == nil || !os.IsNotExist(err) {
			time.Sleep(10 * time.Second)
			continue
		}

		if cidr == "" {
			cidr, err = podCIDR(client, nodeName)
			if err != nil {
				continue
			}
		}

		glog.Infof("Installing CNI on %q", nodeName)

		cniConfig := fmt.Sprintf(cniConfigTemplate, cidr)
		if err := ioutil.WriteFile(cniPath, []byte(cniConfig), 0644); err != nil {
			glog.Errorf("failed to write CNI config %q to %q: %v", cniConfig, cniPath, err)
			continue
		}
	}
}

func clusterConfig(host string) *rest.Config {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Errorf("failed to create in-cluster config: %v", err)
		config.Host = host
	}
	return config
}

func k8sClient() (*kubernetes.Clientset, error) {
	config := clusterConfig("https://localhost:443/")

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Errorf("failed to create the K8s client: %v", err)
		return nil, err
	}
	return client, nil
}

func podCIDR(client *kubernetes.Clientset, nodeName string) (string, error) {
	node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("failed to get node %q's spec: %v", nodeName, err)
		return "", err
	}

	if node.Spec.PodCIDR == "" {
		err = fmt.Errorf("not found podCIDR for node %q", nodeName)
		return "", err
	}

	return node.Spec.PodCIDR, nil
}