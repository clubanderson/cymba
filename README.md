# cymba

A [kcp](https://github.com/kcp-dev/kcp)-based API server for podman hosts.

## Prereqs

- podman version 3.2 or higher installed.
- go version 1.17 or higher
- kubectl installed
- following packages installed (to compile): device-mapper-devel gcc gpgme-devel btrfs-progs-devel 

Start podman system service: (more details [here](https://podman.io/blogs/2020/08/10/podman-go-bindings.html))

```shell
systemctl --user start podman.socket
```

make sure the service is active:

```shell
systemctl --user status podman.socket
```

## Quick Start

Clone this project:

```shell
git clone https://github.com/pdettori/cymba.git
```

Build the binary:

```shell
cd cymba
go build -o  bin/cymba cmd/cymba.go
```

Run the server and controllers:

```shell
bin/cymba
```

Open a different terminal in the cymba directory, and set the KUBECONFIG:

```shell
export KUBECONFIG=.kcp/admin.kubeconfig
```

Check pods in podman:

```shell
podman pod ls
POD ID  NAME    STATUS  CREATED  INFRA ID  # OF CONTAINERS
```

(in this case I have none yet)

Create a sample deployment:

```shell
kubectl apply -f config/samples/deployment.yaml
```

Now check the status of the deployment and pods:

```shell
kubectl get deployment

NAME         READY   UP-TO-DATE   AVAILABLE   AGE
deployment   2/2     2            2           17s
```

```shell
kubectl get pods

NAME               READY   STATUS    RESTARTS   AGE
deployment-228fs   0/1     Running   0          23s
deployment-tkf8f   0/1     Running   0          23s
```

Check that pods are created in podman:

```shell
podman pod ls

POD ID        NAME                       STATUS   CREATED             INFRA ID      # OF CONTAINERS
6b2f001264b2  default_deployment-228fs  Running  2 minutes ago  3a5ccfb7b1ce  2
39ae8b2081eb  default_deployment-tkf8f  Running  2 minutes ago  6983e5a785c8  2
```

## Cleanup

Run `kubectl delete deployment --all` to remove the deployment and all pods


## Installing on Fedora

Tested on Fedora 33 & 34 (Cloud edition):

Install latest version of go (1.17+) following the official [instructions](https://go.dev/doc/install)

Then install packages required for compiling:

```shell
sudo dnf install -y device-mapper-devel gcc gpgme-devel btrfs-progs-devel podman jq
```
