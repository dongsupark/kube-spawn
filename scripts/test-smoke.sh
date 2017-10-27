#!/bin/bash

# Script to test kube-spawn

set -eux
set -o pipefail

CDIR=$(cd "$(dirname "$0")" && pwd)
pushd "$CDIR"
trap 'popd' EXIT

if ! vagrant version > /dev/null 2>&1; then
	# vagrant 2.0 is necessary, because the default vagrant 1.4 on Ubuntu 14.04
	# doesn't support some syntax in Vagrantfile (e.g. 'env').
	curl -s -o /tmp/vagrant.deb https://releases.hashicorp.com/vagrant/2.0.0/vagrant_2.0.0_x86_64.deb
	sudo dpkg -i /tmp/vagrant.deb
	rm -f /tmp/vagrant.deb
fi

if [[ ! -f "/sys/module/vboxdrv" ]]; then
	sudo apt-get install -y virtualbox
	sudo modprobe vboxdrv
fi

IMAGENAME="fedora/26-cloud-base"
if ! vagrant box list | grep ${IMAGENAME} > /dev/null 2>&1; then
	vagrant box add ${IMAGENAME} --provider=virtualbox
 fi

MSTATUS="$(vagrant status fedora |grep fedora|awk -F' ' '{print $2}')"
if [[ "${MSTATUS}" == "running" ]]; then
	vagrant halt fedora
fi

vagrant up fedora --provider=virtualbox

vagrant ssh fedora -c " \
	sudo setenforce 0; \
	go get -u github.com/containernetworking/plugins/plugins/... && \
	cd ~/go/src/github.com/kinvolk/kube-spawn && \
	DOCKERIZED=n make all && \
	sudo -E go test -v --tags integration ./tests \
	"
RESCODE=$?
if [[ "${RESCODE}" -eq 0 ]]; then
	RES="SUCCESS"
else
	RES="FAILURE"
fi

echo "Test result: ${RES}"

trap 'vagrant halt fedora' EXIT
