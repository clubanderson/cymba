/*
Copyright 2021 The KCP Authors.

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
	"flag"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/pdettori/cymba/pkg/controllers"
	"github.com/pdettori/cymba/pkg/controllers/deployment"
	"github.com/pdettori/cymba/pkg/controllers/pod"
)

const numThreads = 1

var kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig")
var kubecontext = flag.String("context", "", "Context to use in the Kubeconfig file, instead of the current context")

func main() {
	flag.Parse()

	var overrides clientcmd.ConfigOverrides
	if *kubecontext != "" {
		overrides.CurrentContext = *kubecontext
	}

	r, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: *kubeconfig},
		&overrides).ClientConfig()
	if err != nil {
		klog.Fatal(err)
	}

	//stopCh := make(chan struct{}) // TODO: hook this up to SIGTERM/SIGINT

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := controllers.SetupSignalHandler()

	go deployment.NewController(r, stopCh).Start(numThreads)
	klog.Infof("Deployment controller launched")

	pod.NewController(r, stopCh).Start(numThreads)
	deployment.NewController(r, stopCh).Start(numThreads)

	<-stopCh
	klog.Infof("Stopping workers")
}
