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

Create the default namespace and then a sample deployment:

```shell
kubectl create ns default
kubectl apply -f config/samples/deployment1.yaml
```

Now check the status of the deployment and pods:

```shell
kubectl get deployment

NAME          READY   UP-TO-DATE   AVAILABLE   AGE
deployment1   4/4     4            4           56s
```

```shell
kubectl get pods

NAME                READY   STATUS    RESTARTS   AGE
deployment1-4dnqh   0/1     Running   0          85s
deployment1-9lzrm   0/1     Running   0          85s
deployment1-g7d6h   0/1     Running   0          85s
deployment1-mb6jn   0/1     Running   0          85s
```

Check that pods are created in podman:

```shell
podman pod ls

POD ID        NAME                       STATUS   CREATED             INFRA ID      # OF CONTAINERS
e753f6324a62  default_deployment1-g7d6h  Running  About a minute ago  7ee810576e70  2
1d6f3dc0b3f7  default_deployment1-4dnqh  Running  About a minute ago  050762b3cb7f  2
8a388ef93d83  default_deployment1-9lzrm  Running  About a minute ago  4aa472b875be  2
58e3da059ff3  default_deployment1-mb6jn  Running  About a minute ago  a8a9de9d5d61  2
```

## Cleanup

Run `kubectl delete deployment --all` to remove the deployment and all pods


## Installing on Fedora

Tested on Fedora 33 & 34 (Cloud edition):

Install latest version of go (1.17+) following the official [instructions](https://go.dev/doc/install)

Then install packages required for compiling:

```shell
sudo dnf install -y device-mapper-devel gcc gpgme-devel btrfs-progs-devel podman
```
