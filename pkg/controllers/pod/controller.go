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

package pod

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterclient "github.com/kcp-dev/kcp/pkg/client/clientset/versioned"
	"github.com/kcp-dev/kcp/pkg/client/informers/externalversions"
	"github.com/pdettori/cymba/pkg/podman"
)

const resyncPeriod = 30 * time.Second
const controllerName = "pod"

// NewController returns a new Controller which handles pods
func NewController(cfg *rest.Config, stopCh <-chan struct{}) *Controller {
	client := corev1client.NewForConfigOrDie(cfg)
	kubeClient := kubernetes.NewForConfigOrDie(cfg)
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	csif := externalversions.NewSharedInformerFactoryWithOptions(clusterclient.NewForConfigOrDie(cfg), resyncPeriod)

	c := &Controller{
		queue:      queue,
		client:     client,
		kubeClient: kubeClient,
		stopCh:     stopCh,
	}
	csif.WaitForCacheSync(stopCh)
	csif.Start(stopCh)

	sif := informers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	sif.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueue(obj) },
		UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
	})
	sif.WaitForCacheSync(stopCh)
	sif.Start(stopCh)

	c.indexer = sif.Core().V1().Pods().Informer().GetIndexer()
	c.lister = sif.Core().V1().Pods().Lister()

	connection, err := podman.GetConnection()
	if err != nil {
		os.Exit(1)
	}
	c.pConn = connection

	return c
}

// Controller defines the struct for Controller
type Controller struct {
	queue      workqueue.RateLimitingInterface
	client     *corev1client.CoreV1Client
	kubeClient kubernetes.Interface
	stopCh     <-chan struct{}
	indexer    cache.Indexer
	lister     corev1lister.PodLister
	pConn      context.Context
}

func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// Start starts the controller
func (c *Controller) Start(numThreads int) {
	defer c.queue.ShutDown()
	for i := 0; i < numThreads; i++ {
		go wait.Until(c.startWorker, time.Second, c.stopCh)
	}
	klog.Infof("Starting pod controller workers")
	<-c.stopCh
	klog.Infof("Stopping pod controller workers")
}

func (c *Controller) startWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(key); err != nil {
		runtime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", controllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *Controller) process(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		klog.Infof("Object with key %q was deleted", key)
		return nil
	}
	current := obj.(*corev1.Pod).DeepCopy()
	previous := current.DeepCopy()

	ctx := context.TODO()
	if err := c.reconcile(ctx, current); err != nil {
		return err
	}

	// If the object being reconciled changed as a result, update it.
	if !equality.Semantic.DeepEqual(previous, current) {
		_, uerr := c.client.Pods(current.Namespace).Update(ctx, current, metav1.UpdateOptions{})
		return uerr
	}

	return err
}
