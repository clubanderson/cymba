#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

source ${PROJECT_HOME}/deploy/config.sh

APISERVER_HOME=${PROJECT_HOME}/.kcp

KCP_PORT=6443
CYMBA_IMAGE=quay.io/pdettori/cymba
CYMBA_EXEC=/cymba
REGISTRATION_IMAGE=quay.io/pdettori/registration
REGISTRATION_EXEC=/registration
WORK_IMAGE=quay.io/open-cluster-management/work
WORK_EXEC=/work

###############################################################################################
#               Functions
###############################################################################################

create_dirs() {
  mkdir -p ${APISERVER_HOME}/manifests
  mkdir -p ${APISERVER_HOME}/hub
}

generate_cymba_command() {
   systemctl is-active --quiet cymba.service
   if [ "$?" -eq 0 ]; then
    sudo systemctl stop cymba.service 
   fi
   sudo cp ${PROJECT_HOME}/bin/cymba /usr/local/bin/cymba
   echo /usr/local/bin/cymba
}

create_systemd_unit() {
 description=$1
 after=$2
 user=$(whoami)
 execStart=$3
 env="PODMAN_URL=unix:///run/user/$(id -u $(whoami))/podman/podman.sock"

 cat ${PROJECT_HOME}/deploy/manifests/systemd.service | \
        sed "s|{{ .description }}|${description}|g" |
        sed "s|{{ .after }}|${after}|g" |
        sed "s|{{ .user }}|${user}|g" |
        sed "s|{{ .workingDir }}|${PROJECT_HOME}|g" |
        sed "s|{{ .environment }}|\"${env}\"|g" |
        sed "s|{{ .execStart }}|${execStart}|g" > ${APISERVER_HOME}/manifests/${description}.service
  sudo cp ${APISERVER_HOME}/manifests/${description}.service /etc/systemd/system/${description}.service
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

get_local_ip() {
  ip route get 1 | sed -n 's/^.*src \([0-9.]*\) .*$/\1/p'
}

prepare_manifests() {
   LOCAL_IP=$1
   CLUSTER_NAME=$2
  
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

update_kubeconfig() {
    IP=$1
    PORT=$2
    CURRENT_SERVER=$(cat ${APISERVER_HOME}/admin.kubeconfig | grep server: | awk '{print $2}' | head -n 1)
    INPUT="$(cat ${APISERVER_HOME}/admin.kubeconfig)"
    OUTPUT=${INPUT//"${CURRENT_SERVER}"/https://${IP}:${PORT}}
    echo "${OUTPUT}" > ${APISERVER_HOME}/admin.kubeconfig
}

extract_exec() {
  image=$1
  sourcePath=$2
  mkdir -p ${APISERVER_HOME}/bin
  containerId=$(podman create $image)  
  podman cp ${containerId}:${sourcePath} ${APISERVER_HOME}/bin/${sourcePath}
  podman rm ${containerId}
}

generate_registration_command() {
   clusterName=$1
   systemctl is-active --quiet ocm-registration.service
   if [ "$?" -eq 0 ]; then
    sudo systemctl stop ocm-registration.service
   fi
   sudo cp ${APISERVER_HOME}/bin/registration /usr/local/bin/registration
   echo /usr/local/bin/registration agent \
    --cluster-name=${clusterName} \
    --bootstrap-kubeconfig=${APISERVER_HOME}/bootstrap/kubeconfig \
    --hub-kubeconfig-dir=${APISERVER_HOME}/hub \
    --kubeconfig=${APISERVER_HOME}/admin.kubeconfig \
    --namespace=open-cluster-management-agent
}

generate_work_command() {
   clusterName=$1
   systemctl is-active --quiet ocm-work.service
   if [ "$?" -eq 0 ]; then
    sudo systemctl stop ocm-work.service
   fi
   sudo cp ${APISERVER_HOME}/bin/work /usr/local/bin/work
   echo /usr/local/bin/work agent \
    --spoke-cluster-name=${clusterName} \
    --hub-kubeconfig=${APISERVER_HOME}/hub/kubeconfig \
    --kubeconfig=${APISERVER_HOME}/admin.kubeconfig \
    --namespace=open-cluster-management-agent \
    --listen=0.0.0.0:9443
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

create_dirs

regExec=$(generate_cymba_command)

create_systemd_unit cymba network.target "${regExec}"

sudo systemctl daemon-reload

sudo systemctl enable cymba.service

sudo systemctl start cymba.service

check_control_plane_up

create_boostrap $apiserver $token

get_and_set_hub_ca

local_ip=$(get_local_ip)

prepare_manifests $local_ip $name

configure_control_plane

create_api_server_extension_cm

update_kubeconfig $local_ip $KCP_PORT

extract_exec ${REGISTRATION_IMAGE} ${REGISTRATION_EXEC}

extract_exec ${WORK_IMAGE} ${WORK_EXEC}

regExec=$(generate_registration_command ${name})

create_systemd_unit ocm-registration "network.target cymba.service" "${regExec}"

regExec=$(generate_work_command ${name})

create_systemd_unit ocm-work "network.target ocm-registration.service" "${regExec}"

sudo systemctl daemon-reload

sudo systemctl enable ocm-registration.service
sudo systemctl enable ocm-work.service

sudo systemctl start ocm-registration.service
sudo systemctl start ocm-work.service