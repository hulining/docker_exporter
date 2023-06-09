#!/bin/bash

[ -n "$DEBUG" ] && set -x
: ${GOARCH:=amd64}
# 脚本要存放在项目根目录
readonly PRO_ROOT=$(cd $(dirname ${BASH_SOURCE:-$0})/../; pwd -P)
source "${PRO_ROOT}/build/lib/var.sh"

read TAG_NUM LDFLAGS < <(BUILD::SetVersion)

app_name="docker_exporter"

echo CGO_ENABLED=0 GOARCH=${GOARCH} go build -o ${PRO_ROOT}/${app_name} -ldflags "${LDFLAGS}" ${PRO_ROOT}/main.go


case "$1" in
#  "release") # checkout到tag构建完再checkout回来
#    bash ${PRO_ROOT}/build/lib/all-release.sh
#    ;;
  "build") #使用master构建测试版本
    if [ -z `command -v go ` ];then
      echo go is not in PATH
      exit 1
    fi
    CGO_ENABLED=0 GOARCH=${GOARCH} go build -o ${PRO_ROOT}/${app_name} -ldflags "${LDFLAGS}" ${PRO_ROOT}/main.go
    ;;
  "docker-local") #使用本地编译二进制文件打包docker
    Dockerfile=Dockerfile.local
    CGO_ENABLED=0 GOARCH=${GOARCH} go build -o ${PRO_ROOT}/${app_name} -ldflags "${LDFLAGS}" ${PRO_ROOT}/main.go
    ;&
  "docker") #使用容器编译和打包
    docker build -t hulining/docker_exporter:$TAG_NUM $build_arg \
      --build-arg LDFLAGS="${LDFLAGS}" \
      --build-arg GOARCH=${GOARCH} \
      -f ${Dockerfile:=Dockerfile} .
    [ -n "${DockerUser}" ] && {
      echo "${DockerPass}" | docker login -u "${DockerUser}" --password-stdin
      docker push hulining/${app_name}:$TAG_NUM
    }
    ;;
  "clean")
    rm -f main
    ;;
  *)
    echo -e "\t\033[1;31m must choose one to run \033[0m"
    exit 1
    ;;
esac
