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

package crd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

const crdPath = "config/crd/bases"

//ApplyCRDs applies the CRDs defined in base path to the embedded API server
func ApplyCRDs(ctx context.Context, config *rest.Config) error {
	apiextensionsClientSet, err := apiextensionsclient.NewForConfig(config)

	crdList, err := readFiles(crdPath)
	if err != nil {
		return err
	}

	crdObjList := []*apiextensions.CustomResourceDefinition{}
	for _, crd := range crdList {
		crdObj, err := deserializeCRD(crd)
		if err != nil {
			return err
		}
		crdObjList = append(crdObjList, crdObj)
		_, err = apiextensionsClientSet.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crdObj, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	err = waitAllCrdRegistered(ctx, apiextensionsClientSet, crdObjList)
	if err != nil {
		return err
	}
	// add extra time for CRD registration (without it still might fail to start controllers)
	time.Sleep(5 * time.Second)

	return nil
}

func readFiles(path string) ([][]byte, error) {
	var fileList = [][]byte{}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() {
			buffer, err := os.ReadFile(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, err
			}
			fileList = append(fileList, buffer)
		}
	}
	return fileList, nil
}

func deserializeCRD(crd []byte) (*apiextensions.CustomResourceDefinition, error) {
	scheme := runtime.NewScheme()
	err := apiextensions.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	factory := serializer.NewCodecFactory(scheme)
	decoder := factory.UniversalDeserializer()

	obj, _, err := decoder.Decode(crd, nil, nil)
	if err != nil {
		return nil, err
	}

	crdObj, ok := obj.(*apiextensions.CustomResourceDefinition)
	if !ok {
		return nil, fmt.Errorf("interface conversion: interface is %T, not *apiextensions.CustomResourceDefinition", obj)
	}

	return crdObj, nil
}

func waitAllCrdRegistered(ctx context.Context, apiextensionsClientSet *apiextensionsclient.Clientset, crdObjList []*apiextensions.CustomResourceDefinition) error {
	checkMap := map[string]bool{}
	for _, crd := range crdObjList {
		checkMap[crd.Name] = true
	}

	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		// Get does not work with KCP, it returns Cluster should not be empty for request 'get' on resource 'customresourcedefinitions'
		// likely it's because cluster header is not inserted for "get" https://github.com/davidfestal/kcp-kubernetes/blob/ff838d2deed553d8a6923dee064cbebb799ba281/pkg/controlplane/clientutils/multiclusterconfig.go
		list, err := apiextensionsClientSet.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, crdObj := range list.Items {
			if _, ok := checkMap[crdObj.Name]; ok {
				for _, cond := range crdObj.Status.Conditions {
					switch cond.Type {
					case apiextensions.NamesAccepted:
						if cond.Status == apiextensions.ConditionTrue {
							delete(checkMap, crdObj.Name)
						}
					}
				}

			}
		}
		if len(checkMap) == 0 {
			return true, nil
		}
		return false, nil
	})
	return err
}
