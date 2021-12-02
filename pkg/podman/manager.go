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

package podman

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/images"
	"github.com/containers/podman/v3/pkg/bindings/pods"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dockerRegistry = "docker.io"
)

// GetConnection - gets a connection to the podman server via socket
func GetConnection() (context.Context, error) {
	// Get Podman socket location
	socket, present := os.LookupEnv("PODMAN_URL")
	if !present {
		sockDir := os.Getenv("XDG_RUNTIME_DIR")
		socket = "unix:" + sockDir + "/podman/podman.sock"
	}

	// Connect to Podman socket
	return bindings.NewConnection(context.Background(), socket)
}

// Gets FQ name for images such as images from docker hub
func getImageFQName(name string) string {
	fqname := name
	if !strings.Contains(name, "/") {
		fqname = dockerRegistry + "/" + name
	}
	return fqname
}

// CreatePod creates and runs a pod with podman from a corev1.PodSpec
func CreatePod(ctx context.Context, p *corev1.Pod) (*entities.PodCreateReport, error) {
	podSpec := specgen.NewPodSpecGenerator()
	podSpec.Name = p.Namespace + "_" + p.Name
	//cOpts := &pods.CreateOptions{}
	//pr, err := pods.CreatePodFromSpec(ctx, podSpec, cOpts)
	// TODO - fix this
	pr, err := pods.CreatePodFromSpec(ctx, nil)
	if err != nil {
		return nil, err
	}
	_, err = pods.Start(ctx, pr.Id, nil)
	if err != nil {
		return nil, err
	}

	for _, container := range p.Spec.Containers {
		image := getImageFQName(container.Image)
		// TBD - add correct handling for IfNotPresent policy
		if container.ImagePullPolicy != "Never" {
			_, err := images.Pull(ctx, image, &images.PullOptions{})
			if err != nil {
				return nil, err
			}
		}

		// Container create
		s := specgen.NewSpecGenerator(image, false)
		s.Terminal = false
		s.Name = podSpec.Name + "_" + container.Name
		s.Pod = pr.Id
		s.Command = container.Command
		// TODO look into networking & ports setup
		r, err := containers.CreateWithSpec(ctx, s, &containers.CreateOptions{})
		if err != nil {
			return nil, err
		}

		// Container start
		err = containers.Start(ctx, r.ID, nil)
		if err != nil {
			return nil, err
		}
	}

	return pr, nil
}

// GetPod gets info about a pod
func GetPod(ctx context.Context, p *corev1.Pod) (*entities.PodInspectReport, error) {
	name := p.Namespace + "_" + p.Name
	return pods.Inspect(ctx, name, &pods.InspectOptions{})
}

// GetPodStatus gets pod info and fills corev1.Pod
func GetPodStatus(ctx context.Context, p *corev1.Pod) error {
	pr, err := GetPod(ctx, p)
	if err != nil {
		return err
	}
	p.Status.Phase = corev1.PodPhase(pr.State)
	t := metav1.NewTime(pr.Created)
	p.Status.StartTime = &t
	p.Status.ContainerStatuses = []corev1.ContainerStatus{}
	for _, c := range pr.Containers {
		cStatus := corev1.ContainerStatus{
			Name: c.Name,
			// TODO - this would require a function and more work
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.NewTime(pr.Created), // TODO
			}},
			ContainerID: c.ID,
			Image:       "tbd",
			ImageID:     "tbd",
		}
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, cStatus)
	}
	// TODO - this requires much more work, currently just a mock
	p.Status.Conditions = []corev1.PodCondition{
		{
			Type:               corev1.PodReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(pr.Created),
			LastProbeTime:      metav1.NewTime(time.Now()),
		},
	}
	return nil
}

// RemovePod deletes a pod and all containers in the pod
func RemovePod(ctx context.Context, p *corev1.Pod) (*entities.PodRmReport, error) {
	name := p.Namespace + "_" + p.Name
	_, err := pods.Kill(ctx, name, &pods.KillOptions{})
	if err != nil {
		return nil, err
	}
	forceRm := true
	return pods.Remove(ctx, name, &pods.RemoveOptions{Force: &forceRm})
}

// IsPodNotFound parses podman error message to check if a pod was not found
func IsPodNotFound(err error) bool {
	return strings.Contains(err.Error(), "no such pod")
}
