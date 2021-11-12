/*
Copyright 2021.

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	podOwnerKey = ".metadata.controller"
	apiGVStr    = schema.GroupVersion{Group: "apps", Version: "v1"}.String()
)

// name of our custom finalizer
const deployFinalizer = "controller.deployment.kcp.dev/finalizer"

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	CSet   *kubernetes.Clientset
}

// Reconcile -
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deployment", req.NamespacedName)

	var deploy appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deploy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Got deployment", "deployment", req.Name)

	// check current status (does pod exist ?)
	var childPods corev1.PodList
	if err := r.List(ctx, &childPods, client.InNamespace(req.Namespace), client.MatchingFields{podOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Pods")
		return ctrl.Result{}, err
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if deploy.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(deploy.GetFinalizers(), deployFinalizer) {
			controllerutil.AddFinalizer(&deploy, deployFinalizer)
			_, err := r.CSet.AppsV1().Deployments(deploy.Namespace).Update(ctx, &deploy, v1.UpdateOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(deploy.GetFinalizers(), deployFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			for _, pod := range childPods.Items {
				log.Info("Attempting to delete pod:", "name", pod.Name)
				if err := r.CSet.CoreV1().Pods(deploy.Namespace).Delete(ctx, pod.Name, v1.DeleteOptions{}); err != nil {
					// if fail to delete the external dependency here, return with error
					// so that it can be retried
					log.Error(err, "Error deleting pod", "name", pod.Name)
					return ctrl.Result{}, err
				}
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&deploy, deployFinalizer)
			_, err := r.CSet.AppsV1().Deployments(deploy.Namespace).Update(ctx, &deploy, v1.UpdateOptions{})
			if err != nil {
				return ctrl.Result{}, IgnoreConflict(err)
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	actual := len(childPods.Items) // TODO make sure you do not include pods being terminated here
	desired := int(*deploy.Spec.Replicas)

	if desired > actual {
		n := desired - actual
		log.Info("Need to create new pods", "n", n)
		for i := 0; i < n; i++ {
			log.Info("Creating pod replica", "#", i)
			p := genPodSpec(deploy)
			_, err := r.CSet.CoreV1().Pods(deploy.Namespace).Create(ctx, &p, v1.CreateOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	} else if desired < actual {
		n := actual - desired
		log.Info("Need to remove pods", "n", n)
		for i := 0; i < n; i++ {
			log.Info("Deleting pod replica", "#", i)
			p := childPods.Items[i]
			err := r.CSet.CoreV1().Pods(deploy.Namespace).Delete(ctx, p.Name, v1.DeleteOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	// if desired == actual we are all happy

	updateDeployStatus(childPods, &deploy)
	// using the controller runtime client with deploy to update the status generated an error
	_, err := r.CSet.AppsV1().Deployments(deploy.Namespace).UpdateStatus(ctx, &deploy, v1.UpdateOptions{})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	config := ctrl.GetConfigOrDie()
	r.CSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, podOwnerKey, func(rawObj client.Object) []string {
		// grab the job object, extract the owner...
		pod := rawObj.(*corev1.Pod)
		owner := metav1.GetControllerOf(pod)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "Deployment" {
			return nil
		}
		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// genPodSpec generates the spec for each pod
func genPodSpec(d appsv1.Deployment) corev1.Pod {
	p := corev1.Pod{
		TypeMeta: v1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			GenerateName: d.Name + "-",
			Namespace:    d.Namespace,
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion:         d.APIVersion,
					Kind:               d.Kind,
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

func updateDeployStatus(pods corev1.PodList, deploy *appsv1.Deployment) {
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
