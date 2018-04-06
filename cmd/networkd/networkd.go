/*
Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

func main() {
	nodeName, err := os.Hostname()
	if err != nil {
		glog.Error("failed to get node name: %v", err)
		return
	}

	cniPath, cniConfigTemplate := os.Getenv("NETWORKD_CNI_CONFIG_PATH"), os.Getenv("NETWORKD_CNI_NETWORK_CONFIG_TEMPLATE")
	if len(cniPath) == 0 || len(cniConfigTemplate) == 0 {
		glog.Warningf("failed to read either env NETWORKD_CNI_CONFIG_PATH: %q or env NETWORKD_CNI_NETWORK_CONFIG_TEMPLATE: %q", cniPath, cniConfigTemplate)
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

func k8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Errorf("failed to create in-cluster config: %v", err)
		return nil, err
	}

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
