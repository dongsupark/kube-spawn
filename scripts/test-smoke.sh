#!/bin/bash

# Script to run smoke tests of kube-spawn
#
# This script is meant to be running on SemaphoreCI (Ubuntu 14.04).
# it includes lots of workarounds to be able to run vagrant-libvirt on
# an old distro Ubuntu 14.04. So many of them should be removed later
# when testing platforms based on newer distros are available on SemaphoreCI.

set -eux
set -o pipefail

CDIR=$(cd "$(dirname "$0")" && pwd)
pushd "$CDIR"
trap 'popd' EXIT

if ! vagrant version > /dev/null 2>&1; then
	# we need vagrant 1.5 or newer to be able to use vagrant-libvirt,
	# but only 1.4 is installed on Ubuntu 14.04. So we need to manually
	# install the most recent version of vagrant.
	wget https://releases.hashicorp.com/vagrant/2.0.0/vagrant_2.0.0_x86_64.deb -O /tmp/vagrant.deb
	sudo dpkg -i /tmp/vagrant.deb
	rm -f /tmp/vagrant.deb
fi

if [[ -z "$(vagrant plugin list | grep vagrant-libvirt)" ]]; then
	sudo apt-get update
	sudo apt-get install -y libvirt-dev ruby-dev

	# we need vagrant-libvirt 0.0.35 or older to avoid errors like
	# "dhcp_leases: undefined method" with libvirt < 1.2.6.
	# (see https://github.com/vagrant-libvirt/vagrant-libvirt/issues/669)
	vagrant plugin install vagrant-libvirt --plugin-version 0.0.35
fi

sudo apt-get install -y libvirt-bin qemu
sudo chgrp -R libvirtd /dev/net/tun

# $USER must belong to group "libvirtd" to be able to run virsh commands.
# $USER must belong to group "kvm" to be able to access to /dev/kvm.
# It's necessary for spawning a new shell with the username, as the existing
# shell would still not have the group privilege of libvirtd & kvm.
# This shell should therefore run following virsh commands inside the spawned
# shell, by running with commands like: sudo su - $USER -c "/some/command"
sudo gpasswd -a $USER libvirtd
sudo gpasswd -a $USER kvm

sudo su - $USER -c "virsh pool-define /dev/stdin <<EOF
<pool type='dir'>
  <name>default</name>
  <target>
    <path>/var/lib/libvirt/images</path>
  </target>
</pool>
EOF"

TOPSRC="$PWD"

sudo su - $USER -c "cd ${TOPSRC}; \
virsh pool-start default; \
virsh pool-autostart default; \
vagrant up fedora --provider=libvirt; \
vagrant ssh fedora -c \" \
	sudo setenforce 0; \
	go get -u github.com/containernetworking/plugins/plugins/... && \
	cd ~/go/src/github.com/kinvolk/kube-spawn && \
	make dep vendor all && \
	sudo -E go test -v --tags integration ./tests \
	\"; \
RESCODE=$?; \
[[ \"\$RESCODE\" -eq 0 ]] && RES=\"SUCCESS\" || RES=\"FAILURE\"; \
echo \"Test result: \$RES\"; \
trap 'vagrant halt fedora' EXIT; \
"
