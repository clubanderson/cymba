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
  podman pod rm -f agent-kcp &> /dev/null
}

###########################################################################################
#                   Main   
###########################################################################################

delete_dir

delete_agent

