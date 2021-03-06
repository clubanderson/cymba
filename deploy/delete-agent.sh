#!/bin/bash

SCRIPT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

###############################################################################################
#               Functions
###############################################################################################

set_home() {
  echo ${PROJECT_HOME} | grep cymba > /dev/null
  if [ "$?" -ne 0 ]; then 
    echo "not running in context"
    APISERVER_HOME=${HOME}/.kcp
    PROJECT_HOME=${HOME}
    IN_CTX=false
  else  
    echo "running in context"
    APISERVER_HOME=${PROJECT_HOME}/.kcp
  fi  
}

delete_dir() {
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
  sudo systemctl reset-failed
}


###########################################################################################
#                   Main   
###########################################################################################

if [[ "$OSTYPE" == "darwin"* ]]; then
  echo "running on macOS"
  if [ "$SSH_CMD" == "" ]; then
    echo "Env var SSH_CMD must be set"
    exit -1
  fi
  $SSH_CMD 'bash -s' < ${SCRIPT_HOME}/delete-agent.sh "$@"
  exit 0
fi  

# handle linux remote to linux
if [ "$SSH_CMD" != "" ]; then
  $SSH_CMD 'bash -s' < ${SCRIPT_HOME}/delete-agent.sh "$@"
  exit 0
fi

set_home

delete_agent

stop_system_services

disable_system_services

remove_system_services

delete_dir


