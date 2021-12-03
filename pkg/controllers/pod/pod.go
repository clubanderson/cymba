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

	"github.com/pdettori/cymba/pkg/controllers"
	"github.com/pdettori/cymba/pkg/podman"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	podFinalizer = "controller.podman.kcp.dev/finalizer"
)

func (c *Controller) reconcile(ctx context.Context, pod *corev1.Pod) error {
	klog.Infof("reconciling pod %q", pod.Name)

	// examine DeletionTimestamp to determine if object is under deletion
	if pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllers.ContainsString(pod.GetFinalizers(), podFinalizer) {
			controllerutil.AddFinalizer(pod, podFinalizer)
			_, err := c.client.Pods(pod.Namespace).Update(ctx, pod, v1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	} else {
		// The object is being deleted
		if controllers.ContainsString(pod.GetFinalizers(), podFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			_, err := podman.RemovePod(c.pConn, pod)
			if err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(pod, podFinalizer)
			_, err = c.client.Pods(pod.Namespace).Update(ctx, pod, v1.UpdateOptions{})
			if err != nil {
				return err
			}
		}

		// Stop reconciliation as the item is being deleted
		return nil
	}

	// check current status (does pod exist ?)
	if err := podman.GetPodStatus(c.pConn, pod); err != nil {
		klog.Info("Error getting pod", "error", err)
		if podman.IsPodNotFound(err) {
			// create pod
			_, err = podman.CreatePod(c.pConn, pod)
			if err != nil {
				return err
			}
			return nil
		}
		// requeue if any other error
		return err
	}

	// using the controller runtime client with pod to update the status generated an error
	_, err := c.client.Pods(pod.Namespace).UpdateStatus(ctx, pod, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
