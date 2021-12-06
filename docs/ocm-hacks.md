# Experiments with OCM agents

How to start OCM registration agent with a KCP server (work in progress)

```
export KUBECONFIG=/kubeconfig/admin.kubeconfig

# find uid 
podman run -v /home/ubuntu/.kcp:/kubeconfig:Z --env KUBECONFIG -it quay.io/open-cluster-management/registration id -u

# in this case it's 10001

# run the podman unshare
podman unshare chown 10001:10001 -R /home/ubuntu/.kcp


podman run -v /home/ubuntu/.kcp:/kubeconfig:Z --env KUBECONFIG --network="host" quay.io/open-cluster-management/registration /registration agent --cluster-name=cluster1 --kubeconfig=$KUBECONFIG
```