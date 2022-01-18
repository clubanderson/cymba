#!/bin/bash

PROJECT_HOME="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

podman machine ssh 'bash -s' < ${PROJECT_HOME}/deploy/delete-agent.sh "$@"


