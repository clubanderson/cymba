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

package deployment

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/pdettori/cymba/pkg/controllers"
)

const (
	deployFinalizer = "controller.deployment.kcp.dev/finalizer"
	ownedByLabel    = "kcp.dev/owned-by"
)

func (c *Controller) reconcile(ctx context.Context, deployment *appsv1.Deployment) error {
	klog.Infof("reconciling deployment %q", deployment.Name)

	// check current status (does pod exist ?)
	childPods, err := c.kubeClient.CoreV1().Pods(deployment.Namespace).List(ctx, v1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", ownedByLabel, deployment.Name)})
	if err != nil {
		klog.Error(err, "unable to list child Pods")
		return err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if deployment.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllers.ContainsString(deployment.GetFinalizers(), deployFinalizer) {
			controllerutil.AddFinalizer(deployment, deployFinalizer)
			_, err := c.client.Deployments(deployment.Namespace).Update(ctx, deployment, v1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	} else {
		// The object is being deleted
		if controllers.ContainsString(deployment.GetFinalizers(), deployFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			for _, pod := range childPods.Items {
				klog.Info("Attempting to delete pod:", "name", pod.Name)
				if err := c.kubeClient.CoreV1().Pods(deployment.Namespace).Delete(ctx, pod.Name, v1.DeleteOptions{}); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					klog.Error(err, "Error deleting pod", "name", pod.Name)
					return err
				}
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(deployment, deployFinalizer)
			_, err := c.client.Deployments(deployment.Namespace).Update(ctx, deployment, v1.UpdateOptions{})
			if err != nil {
				return IgnoreConflict(err)
			}
		}

		// Stop reconciliation as the item is being deleted
		return nil
	}

	actual := len(childPods.Items) // TODO make sure you do not include pods being terminated here
	desired := int(*deployment.Spec.Replicas)

	if desired > actual {
		n := desired - actual
		klog.Info("Need to create new pods", "n", n)
		for i := 0; i < n; i++ {
			klog.Info("Creating pod replica", "#", i)
			p := genPodSpec(deployment)
			_, err := c.kubeClient.CoreV1().Pods(deployment.Namespace).Create(ctx, &p, v1.CreateOptions{})
			if err != nil {
				return err
			}
		}
		return nil
	} else if desired < actual {
		n := actual - desired
		klog.Info("Need to remove pods", "n", n)
		for i := 0; i < n; i++ {
			klog.Info("Deleting pod replica", "#", i)
			p := childPods.Items[i]
			err := c.kubeClient.CoreV1().Pods(deployment.Namespace).Delete(ctx, p.Name, v1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
		return nil
	}
	// if desired == actual we are all happy

	updateDeployStatus(childPods, deployment)

	_, err = c.client.Deployments(deployment.Namespace).UpdateStatus(ctx, deployment, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// genPodSpec generates the spec for each pod
func genPodSpec(d *appsv1.Deployment) corev1.Pod {
	klog.Infof("Generating pod spec for %s %s %s", d.APIVersion, d.Kind, d.Name)
	p := corev1.Pod{
		TypeMeta: v1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			GenerateName: d.Name + "-",
			Namespace:    d.Namespace,
			Labels:       map[string]string{ownedByLabel: d.Name},
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion:         "apps/v1",
					Kind:               "Deployment",
					Name:               d.Name,
					UID:                d.UID,
					Controller:         bool2ptr(true),
					BlockOwnerDeletion: bool2ptr(true),
				},
			},
		},
		Spec: d.Spec.Template.Spec,
	}
	return p
}

func updateDeployStatus(pods *corev1.PodList, deploy *appsv1.Deployment) {
	deploy.Status.ObservedGeneration = deploy.Generation
	var availableReplicas int32
	var readyReplicas int32
	var replicas int32
	var updatedReplicas int32

	for _, p := range pods.Items {
		replicas++
		if p.Status.Phase == "Running" {
			// TODO - this will need some revisiting
			availableReplicas++
			readyReplicas++
			updatedReplicas++
		}
	}
	deploy.Status.AvailableReplicas = availableReplicas
	deploy.Status.ReadyReplicas = readyReplicas
	deploy.Status.Replicas = replicas
	deploy.Status.UpdatedReplicas = updatedReplicas
}

func bool2ptr(b bool) *bool {
	return &b
}

// IgnoreConflict returns nil on Conflict errors, originating from updating when finalizer is deleted
// this is not a good practice but ok for finalizer here
func IgnoreConflict(err error) error {
	if apierrors.IsConflict(err) {
		return nil
	}
	return err
}
