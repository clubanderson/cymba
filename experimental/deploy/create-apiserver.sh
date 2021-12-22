#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/../.. && pwd )"

source ${PROJECT_HOME}/experimental/deploy/config.sh

###############################################################################################
#               Functions
###############################################################################################



create_certs() {
    IP=$1
    mkdir -p ${APISERVER_HOME}/pki
    rm ${APISERVER_HOME}/*.conf &>/dev/null
    kubeadm init phase certs --cert-dir=${APISERVER_HOME}/pki all
    kubeadm init phase kubeconfig --cert-dir=${APISERVER_HOME}/pki --kubeconfig-dir=${APISERVER_HOME} admin
    kubeadm init phase kubeconfig --cert-dir=${APISERVER_HOME}/pki --kubeconfig-dir=${APISERVER_HOME} controller-manager
}

create_manifests() {
    tmp_dir=$(mktemp -d -t kubeadm-XXXXXXXXXX)
    mkdir -p ${APISERVER_HOME}/manifests
    sudo kubeadm init phase control-plane --rootfs ${tmp_dir} apiserver &>/dev/null
    sudo kubeadm init phase control-plane --rootfs ${tmp_dir} controller-manager &>/dev/null
    sudo kubeadm init phase etcd local --rootfs ${tmp_dir} --cert-dir=${APISERVER_HOME}/pki &>/dev/null
    sudo cp ${tmp_dir}/etc/kubernetes/manifests/kube-apiserver.yaml ${APISERVER_HOME}/manifests/
    sudo cp ${tmp_dir}/etc/kubernetes/manifests/kube-controller-manager.yaml ${APISERVER_HOME}/manifests/
    sudo cp ${tmp_dir}/etc/kubernetes/manifests/etcd.yaml ${APISERVER_HOME}/manifests/
    sudo chown $(logname) ${APISERVER_HOME}/manifests/*.yaml
    sudo rm -rf ${tmp_dir}
}


###########################################################################################
#                   Main   
###########################################################################################

# create_certs

create_manifests