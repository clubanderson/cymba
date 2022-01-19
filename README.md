# cymba

A [kcp](https://github.com/kcp-dev/kcp)-based API server for podman hosts, integrated with
[Open Cluster Management](https://open-cluster-management.io) (OCM) agents.

## Quick Start

Clone this project:

```shell
git clone https://github.com/pdettori/cymba.git
```

You may deploy cymba on a local host or remotely via SSH. The target host should have podman installed.
To deploy cymba with the provided installation script, you'll need first an instance of an OCM hub
running. You may follow the [Quick Start](https://open-cluster-management.io/getting-started/quick-start/)
insstructions to get quickly a hub instance running. You may also try cymba with [kealm](https://github.com/pdettori/kealm), which allows to create virtual OCM hub instances on a kubernetes cluster and provides additional
application lifecycle managememt abstrations.

When you create a OCM hub instance, with OCM or kealm, you'll get printout with an hub token and hub URL you can use to
register an agent with the hub. This will be in the format:

```shell
clusteradm join --hub-token <token> --hub-apiserver <api-server-url> --cluster-name <cluster-name>
```

You can use the same format to deploy cymba, by simply running the command:

```shell
deploy/deploy-agent.sh join --hub-token <token> --hub-apiserver <api-server-url> --cluster-name <cluster-name>
```

If you are running on a linux OS, the above command will deploy cymba and the OCM agents on the local host.
If you are running on macOS or want to deploy to a remote host, follow the steps for remote deployment.


### Remote deployment

You may deploy on a remote host by setting up the environment variable `SSH_CMD` for the ssh command used to
connect to the remote host, and then run the deployment command as above.

e.g.

```
export SSH_CMD="ssh fedora@1.2.3.4"
deploy/deploy-agent.sh join --hub-token <token> --hub-apiserver <api-server-url> --cluster-name <cluster-name>
```

On a macOS, you may test cymba with [podman machine](https://medium.com/@AbhijeetKasurde/running-podman-machine-on-macos-1f3fb0dbf73d), which creates a lighteweight Fedora-CoreOS VM on your mac. You may connect to podman machine with the command `podman machine ssh`, so to deploy cymba and register it with
OCM you may just run:

```
export SSH_CMD="podman machine ssh"
deploy/deploy-agent.sh join --hub-token <token> --hub-apiserver <api-server-url> --cluster-name <cluster-name>
```

## Checking status of deployment

The installation script creates three systemd services (`cymba`,`ocm-registration`,`ocm-work`). You may check
the status and logs with the `systemctl` and `journalctl` commands, for example:

```shell
systemctl status cymba
journalctl -u cymba
```

## Deploying workloads on podman with OCM

Once the agents are started on the podman host, you may follow the steps described [here](https://github.com/pdettori/kealm) or in [OCM docs](https://open-cluster-management.io/concepts/) (depending on which hub you used for registration) to accept the registration of the podman host and deploy workloads. Note that at this time you may only
deploy deployments and pods.

## Developement 

### Prereqs

- podman version 3.2 or higher installed.
- go version 1.17 or higher
- kubectl installed
- following packages installed (to compile): gcc  

### Building and testing

Start podman system service: (more details [here](https://podman.io/blogs/2020/08/10/podman-go-bindings.html))

```shell
systemctl --user start podman.socket
```

make sure the service is active:

```shell
systemctl --user status podman.socket
```

Build the binary:

```shell
make build
```

Run the server and controllers:

```shell
make run
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

### Cleanup

Run `kubectl delete deployment --all` to remove the deployment and all pods


### Building on Fedora

Tested on Fedora 33 & 34 (Cloud edition):

Install latest version of go (1.17+) following the official [instructions](https://go.dev/doc/install)

Then install packages required for compiling:

```shell
sudo dnf install -y make gcc
```
