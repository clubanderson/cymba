#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

source ${PROJECT_HOME}/deploy/config.sh

###############################################################################################
#               Functions
###############################################################################################

delete_dir() {
  rm -rf ${APISERVER_HOME}
  rm -rf ${PROJECT_HOME}/.kcp
}

delete_agent() {
  podman pod rm -f agent &> /dev/null
}

stop_system_services() {
   systemctl is-active --quiet ocm-work.service
   if [ "$?" -eq 0 ]; then
    sudo systemctl stop ocm-work.service
   fi

   systemctl is-active --quiet ocm-registration.service
   if [ "$?" -eq 0 ]; then
    sudo systemctl stop ocm-registration.service
   fi

   systemctl is-active --quiet cymba.service
   if [ "$?" -eq 0 ]; then
    sudo systemctl stop cymba.service
   fi
}

disable_system_services() {
  sudo systemctl disable cymba.service
  sudo systemctl disable ocm-registration.service
  sudo systemctl disable ocm-work.service
}

remove_system_services() {
  sudo rm /etc/systemd/system/cymba.service
  sudo rm /etc/systemd/system/ocm-registration.service
  sudo rm /etc/systemd/system/ocm-work.service
  sudo systemctl daemon-reload
}


###########################################################################################
#                   Main   
###########################################################################################

delete_dir

delete_agent

stop_system_services

disable_system_services

remove_system_services


