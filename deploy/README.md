# Experiments around using directly k8s API server with podman

Note: we are temprary using the tcp socket, thus to run you need first to start it with:

```
podman system service tcp:172.31.37.22:9999 -t 0 &
```

Note: need jq installed - on Fedora 'dnf install -y jq'

