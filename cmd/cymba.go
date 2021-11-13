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
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"flag"

	"github.com/kcp-dev/kcp/pkg/server"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/pdettori/cymba/pkg/controllermanager"
	"github.com/pdettori/cymba/pkg/crd"
	genericapiserver "k8s.io/apiserver/pkg/server"
)


func main() {
	var startControllerManager bool
	flag.BoolVar(&startControllerManager, "controller-manager", true,
		"start controller manager with server")
	flag.Parse()

	// Setup signal handler for a cleaner shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Kill, os.Interrupt)
	defer cancel()
	srv := server.NewServer(server.DefaultConfig())

	// Register a post-start hook that connects to the api-server
	if startControllerManager {
		srv.AddPostStartHook("connect-to-api", func(context genericapiserver.PostStartHookContext) error {
			err := crd.ApplyCRDs(ctx, context.LoopbackClientConfig)
			if err != nil {
				return err
			}

			err = controllermanager.StartManager(context.LoopbackClientConfig)
			if err != nil {
				log.Fatalf("error starting controller manager: %s", err)
			}

			return nil
		})
	}
	srv.Run(ctx)
}