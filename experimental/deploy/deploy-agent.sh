#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/../.. && pwd )"

source ${PROJECT_HOME}/experimental/deploy/config.sh

###############################################################################################
#               Functions
###############################################################################################


create_certs() {
    mkdir -p ${APISERVER_HOME}/pki
    rm ${APISERVER_HOME}/*.conf &>/dev/null
    kubeadm init phase certs --cert-dir=${APISERVER_HOME}/pki all
    kubeadm init phase kubeconfig --cert-dir=${APISERVER_HOME}/pki --kubeconfig-dir=${APISERVER_HOME} admin
    kubeadm init phase kubeconfig --cert-dir=${APISERVER_HOME}/pki --kubeconfig-dir=${APISERVER_HOME} controller-manager
}

create_boostrap() {
    apiserver=$1
    token=$2
    mkdir -p ${APISERVER_HOME}/bootstrap
    kubectl config set-cluster hub \
    --kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig  \
    --server=$apiserver #\
    #--certificate-authority=${VKS_HOME}/pki/ca.crt

    kubectl config set-credentials bootstrap \
    --kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig  \
    --token=$token

    kubectl config set-context bootstrap \
    --kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig  \
    --cluster=hub \
    --user=bootstrap

    kubectl config use-context bootstrap \
    --kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig
}

create_dirs() {
  mkdir -p ${APISERVER_HOME}/var/lib/etcd
  mkdir -p ${APISERVER_HOME}/hub
}

get_local_ip() {
  ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p'
}

prepare_manifests() {
   LOCAL_IP=$1
   mkdir -p ${APISERVER_HOME}/manifests
   cat ${PROJECT_HOME}/experimental/deploy/manifests/control-plane.yaml | \
        sed "s|{{ .apiserverHome }}|${APISERVER_HOME}|g" |
        sed "s|{{ .localIP }}|${LOCAL_IP}|g" |
        sed "s|{{ .hostPort }}|${HOST_PORT}|g" > ${APISERVER_HOME}/manifests/control-plane.yaml

   cat ${PROJECT_HOME}/experimental/deploy/manifests/agent.yaml | \
        sed "s|{{ .apiserverHome }}|${APISERVER_HOME}|g" |
        sed "s|{{ .podmanPort }}|${PODMAN_PORT}|g" |
        sed "s|{{ .localIP }}|${LOCAL_IP}|g" > ${APISERVER_HOME}/manifests/agent.yaml

   cp ${PROJECT_HOME}/experimental/deploy/manifests/*.crd.yaml ${APISERVER_HOME}/manifests    
}

update_kubeconfig() {
    IP=$1
    PORT=$2
    CURRENT_SERVER=$(cat ${APISERVER_HOME}/admin.conf | grep server: | awk '{print $2}')
    sed "s|${CURRENT_SERVER}|https://${IP}:${PORT}|g" ${APISERVER_HOME}/admin.conf -i""
}

start_control_plane() {
  podman pod rm -f control-plane &> /dev/null
  podman play kube ${APISERVER_HOME}/manifests/control-plane.yaml
}


###########################################################################################
#                   Main   
###########################################################################################

if [ "$#" -ne 7 ]; then
    echo "Usage: $0 join --hub-token <hub token> --hub-apiserver <hub API server URL> --name <managed host name>"
    exit
fi

if [ "$1" != "join" ]; then
    echo "join is the only supported command"
    exit
fi

ARGS=$(getopt -a --options t:a:n: --long "hub-token:,hub-apiserver:,name:" -- "$@")
eval set -- "$ARGS"

while true; do
  case "$1" in
    -t|--hub-token)
      token="$2"
      shift 2;;
    -a|--hub-apiserver)
      apiserver="$2"
      shift 2;;
    -n|--name)
      name="$2"
      shift 2;;  
    --)
      break;;
     *)
      printf "Unknown option %s\n" "$1"
      exit 1;;
  esac
done

create_certs

create_boostrap $apiserver $token

create_dirs

local_ip=$(get_local_ip)

prepare_manifests $local_ip

update_kubeconfig $local_ip $HOST_PORT

start_control_plane
