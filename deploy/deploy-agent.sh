#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

source ${PROJECT_HOME}/deploy/config.sh

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
    --server=$apiserver

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

get_and_set_hub_ca() {
  tmp_dir=$(mktemp -d -t ca-XXXXXXXXXX)
  
  kubectl --kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig \
  --insecure-skip-tls-verify -n kube-public get cm cluster-info -o json | jq -r '.data.kubeconfig' > ${tmp_dir}/kubeconfig
  
  kubectl --kubeconfig=${tmp_dir}/kubeconfig config view --raw -o json | jq -r '.clusters[0].cluster."certificate-authority-data"'| base64 -d > ${tmp_dir}/ca.crt
  
  kubectl config set-cluster hub \
    --kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig  \
    --certificate-authority=${tmp_dir}/ca.crt \
    --embed-certs
    
   rm -rf ${tmp_dir} 
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
   CLUSTER_NAME=$2
   mkdir -p ${APISERVER_HOME}/manifests
   cat ${PROJECT_HOME}/deploy/manifests/control-plane.yaml | \
        sed "s|{{ .apiserverHome }}|${APISERVER_HOME}|g" |
        sed "s|{{ .localIP }}|${LOCAL_IP}|g" |
        sed "s|{{ .hostPort }}|${HOST_PORT}|g" > ${APISERVER_HOME}/manifests/control-plane.yaml

   cat ${PROJECT_HOME}/deploy/manifests/agent.yaml | \
        sed "s|{{ .apiserverHome }}|${APISERVER_HOME}|g" |
        sed "s|{{ .podmanPort }}|${PODMAN_PORT}|g" |
        sed "s|{{ .clusterName }}|${CLUSTER_NAME}|g" |
        sed "s|{{ .localIP }}|${LOCAL_IP}|g" > ${APISERVER_HOME}/manifests/agent.yaml

   cp ${PROJECT_HOME}/deploy/manifests/*.crd.yaml ${APISERVER_HOME}/manifests
   cp ${PROJECT_HOME}/deploy/manifests/*_namespace.yaml ${APISERVER_HOME}/manifests   
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

check_control_plane_up() {
    echo "checking control plane is up..."
    echo "press CTRL+C to exit"
    for (( ; ; ))
    do
        echo -n "."
        kubectl --kubeconfig=${APISERVER_HOME}/admin.conf cluster-info &> /dev/null
        if [ "$?" -eq 0 ]; then
            echo ""
            echo "control plane ready!"
            kubectl --kubeconfig=${APISERVER_HOME}/admin.conf cluster-info
            break
        fi
        sleep 2
    done
}

configure_control_plane() {
  kubectl --kubeconfig=${APISERVER_HOME}/admin.conf apply -f ${APISERVER_HOME}/manifests/0000_00_namespace.yaml
  kubectl --kubeconfig=${APISERVER_HOME}/admin.conf apply -f ${APISERVER_HOME}/manifests/0000_00_appliedmanifestworks.crd.yaml
  kubectl --kubeconfig=${APISERVER_HOME}/admin.conf apply -f ${APISERVER_HOME}/manifests/0000_00_clusters.open-cluster-management.io_clusterclaims.crd.yaml
}

start_agent() {
  podman pod rm -f agent &> /dev/null
  podman play kube ${APISERVER_HOME}/manifests/agent.yaml
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

ARGS=$(getopt -a --options t:a:n: --long "hub-token:,hub-apiserver:,cluster-name:" -- "$@")
eval set -- "$ARGS"

while true; do
  case "$1" in
    -t|--hub-token)
      token="$2"
      shift 2;;
    -a|--hub-apiserver)
      apiserver="$2"
      shift 2;;
    -n|--cluster-name)
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

get_and_set_hub_ca

create_dirs

local_ip=$(get_local_ip)

prepare_manifests $local_ip $name

update_kubeconfig $local_ip $HOST_PORT

start_control_plane

check_control_plane_up

configure_control_plane

start_agent
