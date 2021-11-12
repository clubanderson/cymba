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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podName       = "mypod"
	podNamespacce = "default"
	containerName = "busybox"
	image         = "busybox:1.25"
)

func TestGetConnection(t *testing.T) {
	conn, err := GetConnection()

	assert.NoError(t, err)
	assert.NotNil(t, conn)
}

func TestGetImageFQName(t *testing.T) {
	image1 := "busybox:1.25"
	image2 := "registry.fedoraproject.org/fedora:latest"

	expFq1 := "docker.io/busybox:1.25"
	expoFq2 := image2

	assert.Equal(t, expFq1, getImageFQName(image1))
	assert.Equal(t, expoFq2, getImageFQName(image2))
}

func TestCreatePod(t *testing.T) {
	conn, err := GetConnection()

	assert.NoError(t, err)

	pod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespacce,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: image,
					Command: []string{
						"sleep",
						"86400",
					},
				},
			},
		},
	}

	_, err = CreatePod(conn, pod)
	assert.NoError(t, err)

	pr, err := GetPod(conn, pod)
	assert.NoError(t, err)

	x, _ := json.Marshal(pr)
	fmt.Printf(">>> %s\n", string(x))

	err = GetPodStatus(conn, &pod)
	assert.NoError(t, err)

	x, _ = json.Marshal(pod)
	fmt.Printf(">>> %s\n", string(x))

	_, err = RemovePod(conn, pod)
	assert.NoError(t, err)

}
