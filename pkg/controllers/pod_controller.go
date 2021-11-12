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
	"github.com/pdettori/cymba/pkg/podman"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// name of our custom finalizer
const podFinalizer = "controller.podman.kcp.dev/finalizer"

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	Conn   context.Context
	CSet   *kubernetes.Clientset
}

//+kubebuilder:rbac:groups=cloud.ibm.com,resources=poddevices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cloud.ibm.com,resources=poddevices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cloud.ibm.com,resources=poddevices/finalizers,verbs=update

// Reconcile -
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pod", req.NamespacedName)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(pod.GetFinalizers(), podFinalizer) {
			controllerutil.AddFinalizer(&pod, podFinalizer)
			_, err := r.CSet.CoreV1().Pods(pod.Namespace).Update(ctx, &pod, v1.UpdateOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(pod.GetFinalizers(), podFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			_, err := podman.RemovePod(r.Conn, pod)
			if err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&pod, podFinalizer)
			_, err = r.CSet.CoreV1().Pods(pod.Namespace).Update(ctx, &pod, v1.UpdateOptions{})
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// check current status (does pod exist ?)
	if err := podman.GetPodStatus(r.Conn, &pod); err != nil {
		log.Info("Error getting pod", "error", err)
		if podman.IsPodNotFound(err) {
			// create pod
			_, err = podman.CreatePod(r.Conn, pod)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		// requeue if any other error
		return ctrl.Result{}, err
	}

	// using the controller runtime client with pod to update the status generated an error
	_, err := r.CSet.CoreV1().Pods(pod.Namespace).UpdateStatus(ctx, &pod, v1.UpdateOptions{})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.Conn, err = podman.GetConnection()
	if err != nil {
		return err
	}
	config := ctrl.GetConfigOrDie()
	r.CSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
