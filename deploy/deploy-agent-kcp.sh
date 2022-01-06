#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

source ${PROJECT_HOME}/deploy/config.sh

APISERVER_HOME=${PROJECT_HOME}/.kcp

KCP_PORT=6443

###############################################################################################
#               Functions
###############################################################################################

start_cymba() {
  mkdir -p ${APISERVER_HOME}
  # TODO - this should become a system service 
  # restart if already running
  if test -f "${APISERVER_HOME}/kcp.pid"; then
      echo "kcp pid exists, restarting..."
      pid=$(cat ${APISERVER_HOME}/kcp.pid)
      kill $pid 
  fi
  ${PROJECT_HOME}/bin/cymba &> ${APISERVER_HOME}/kcp.log &
  kcpPid=$!
  echo "KCP started with pid=${kcpPid}"
  echo ${kcpPid} > ${APISERVER_HOME}/kcp.pid
}


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
  mkdir -p ${APISERVER_HOME}/hub
}

prepare_manifests() {
   LOCAL_IP=$1
   CLUSTER_NAME=$2
   mkdir -p ${APISERVER_HOME}/manifests
  
   cat ${PROJECT_HOME}/deploy/manifests/agent-kcp.yaml | \
        sed "s|{{ .apiserverHome }}|${APISERVER_HOME}|g" |
        sed "s|{{ .podmanPort }}|${PODMAN_PORT}|g" |
        sed "s|{{ .clusterName }}|${CLUSTER_NAME}|g" |
        sed "s|{{ .localIP }}|${LOCAL_IP}|g" > ${APISERVER_HOME}/manifests/agent.yaml

   cp ${PROJECT_HOME}/deploy/manifests/*.crd.yaml ${APISERVER_HOME}/manifests
   cp ${PROJECT_HOME}/deploy/manifests/*_namespace.yaml ${APISERVER_HOME}/manifests
   cp ${PROJECT_HOME}/deploy/manifests/_nodes.yaml ${APISERVER_HOME}/manifests   
}

configure_control_plane() {
  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig apply -f ${APISERVER_HOME}/manifests/0000_00_namespace.yaml
  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig apply -f ${APISERVER_HOME}/manifests/0000_01_kcp_namespace.yaml
  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig apply -f ${APISERVER_HOME}/manifests/0000_00_appliedmanifestworks.crd.yaml
  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig apply -f ${APISERVER_HOME}/manifests/0000_00_clusters.open-cluster-management.io_clusterclaims.crd.yaml
  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig apply -f ${APISERVER_HOME}/manifests/_nodes.yaml
}

create_api_server_extension_cm() {
  tmp_dir=$(mktemp -d -t certs-XXXXXXXXXX)
  
  cp ${APISERVER_HOME}/secrets/ca/cert.pem ${tmp_dir}/ca.crt
  cp ${APISERVER_HOME}/secrets/ca/key.pem ${tmp_dir}/ca.key

  kubeadm init phase certs front-proxy-ca --cert-dir=${tmp_dir}

  echo "created in ${tmp_dir}"

  mv ${tmp_dir}/ca.crt ${tmp_dir}/client-ca-file
  mv ${tmp_dir}/front-proxy-ca.crt ${tmp_dir}/requestheader-client-ca-file

  # if present, remove existing cm
  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig delete configmap extension-apiserver-authentication \
   --namespace kube-system &> /dev/null

  kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig create configmap extension-apiserver-authentication \
   --namespace kube-system \
   --from-file=${tmp_dir}/client-ca-file \
   --from-file=${tmp_dir}/requestheader-client-ca-file \
   --from-literal=requestheader-allowed-names='["front-proxy-client"]' \
   --from-literal=requestheader-extra-headers-prefix='["X-Remote-Extra-"]' \
   --from-literal=requestheader-group-headers='["X-Remote-Group"]' \
   --from-literal=requestheader-username-headers='["X-Remote-User"]'
    
  rm -rf ${tmp_dir} 
}

get_local_ip() {
  ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p'
}


update_kubeconfig() {
    IP=$1
    PORT=$2
    CURRENT_SERVER=$(cat ${APISERVER_HOME}/admin.kubeconfig | grep server: | awk '{print $2}' | head -n 1)
    INPUT="$(cat ${APISERVER_HOME}/admin.kubeconfig)"
    OUTPUT=${INPUT//"${CURRENT_SERVER}"/https://${IP}:${PORT}}
    echo "${OUTPUT}" > ${APISERVER_HOME}/admin.kubeconfig

}

# need to unshare dirs for podman
unshare_dirs() {
  cp ${APISERVER_HOME}/admin.kubeconfig ${APISERVER_HOME}/admin.kubeconfig.unshared
  podman unshare chown 10001:10001 -R ${APISERVER_HOME}/bootstrap/
  podman unshare chown 10001:10001 -R ${APISERVER_HOME}/hub/
  podman unshare chown 10001:10001 -R ${APISERVER_HOME}/admin.kubeconfig.unshared
}

check_control_plane_up() {
    echo "checking control plane is up..."
    echo "press CTRL+C to exit"
    for (( ; ; ))
    do
        echo -n "."
        kubectl --kubeconfig=${APISERVER_HOME}/admin.kubeconfig get apiservices &> /dev/null
        if [ "$?" -eq 0 ]; then
            echo ""
            echo "control plane ready!"
            break
        fi
        sleep 2
    done
}

start_agent() {
  podman pod rm -f agent-kcp &> /dev/null
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

start_cymba

check_control_plane_up

create_boostrap $apiserver $token

get_and_set_hub_ca

create_dirs

local_ip=$(get_local_ip)

prepare_manifests $local_ip $name

configure_control_plane

create_api_server_extension_cm

update_kubeconfig $local_ip $KCP_PORT

unshare_dirs

start_agent


