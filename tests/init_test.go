/*
Copyright 2017 Kinvolk GmbH

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +build integration

package tests

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kinvolk/kube-spawn/pkg/utils"
)

const k8sStableVersion string = "1.7.0"

var (
	numNodes       int = 2
	kubeSpawnPath  string
	kubeCtlPath    string
	machineCtlPath string
)

func checkRequirements(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("smoke test requires root privileges")
	}
}

func initPath(t *testing.T) {
	// go one dir upper, from "tests" to the top source directory
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}

	var pwd string
	var err error
	if pwd, err = os.Getwd(); err != nil {
		t.Fatal(err)
	}

	kubeSpawnPath = filepath.Join(pwd, "kube-spawn")

	if kubeSpawnPath, err = exec.LookPath(kubeSpawnPath); err != nil {
		// fall back to an ordinary abspath to kube-spawn
		kubeSpawnPath = "/usr/bin/kube-spawn"
	}

	kubeCtlPath = filepath.Join(pwd, "k8s/kubectl")
	if kubeCtlPath, err = exec.LookPath(kubeCtlPath); err != nil {
		// fall back to an ordinary abspath to kubectl
		kubeCtlPath = "/usr/bin/kubectl"
	}

	machineCtlPath, err = exec.LookPath("machinectl")
	if err != nil {
		// fall back to an ordinary abspath to machinectl
		machineCtlPath = "/usr/bin/machinectl"
	}
}

func initNode(t *testing.T) {
	// If no coreos image exists, just download it
	if _, _, err := runCommand(fmt.Sprintf("%s show-image coreos", machineCtlPath)); err != nil {
		if stdout, stderr, err := runCommand(fmt.Sprintf("%s pull-raw --verify=no %s %s",
			machineCtlPath,
			"https://alpha.release.core-os.net/amd64-usr/current/coreos_developer_container.bin.bz2",
			"coreos",
		)); err != nil {
			t.Fatalf("error running machinectl pull-raw: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
	}
}

func getRunningNodes() ([]string, error) {
	var nodeNames []string

	stdout, stderr, err := runCommand(fmt.Sprintf("%s list --no-legend", machineCtlPath))
	if err != nil {
		return nil, fmt.Errorf("error running machinectl list: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	s := bufio.NewScanner(strings.NewReader(strings.TrimSpace(stdout)))
	for s.Scan() {
		line := strings.Fields(s.Text())
		if len(line) <= 2 {
			continue
		}

		// an example line:
		//  kube-spawn-0 container systemd-nspawn coreos 1478.0.0 10.22.0.130...
		nodeName := strings.TrimSpace(line[0])
		if !strings.HasPrefix(nodeName, "kube-spawn-") {
			continue
		}

		nodeNames = append(nodeNames, nodeName)
	}

	return nodeNames, nil
}

func checkK8sNodes(t *testing.T) {
	stdout, stderr, err := runCommand(fmt.Sprintf("%s get nodes", kubeCtlPath))
	if err != nil {
		t.Fatalf("error running kubectl get nodes: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	outStr := strings.TrimSpace(string(stdout))
	scanner := bufio.NewScanner(strings.NewReader(outStr))
	var numLines int = 0
	for scanner.Scan() {
		if len(strings.TrimSpace(scanner.Text())) == 0 {
			continue
		}
		numLines += 1
	}

	if numLines != numNodes {
		t.Fatalf("got %d nodes, expected %d nodes.\n", numLines, numNodes)
	}
}

func testSetupK8sStable(t *testing.T) {
	if stdout, stderr, err := runCommand(fmt.Sprintf("%s --kubernetes-version=%s setup --nodes=%d",
		kubeSpawnPath, k8sStableVersion, numNodes),
	); err != nil {
		t.Fatalf("error running kube-spawn setup: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	nodes, err := getRunningNodes()
	if err != nil {
		t.Fatalf("error getting list of running nodes: %v\n", err)
	}
	if len(nodes) != numNodes {
		t.Fatalf("got %d nodes, expected %d nodes.\n", len(nodes), numNodes)
	}
}

func testInitK8sStable(t *testing.T) {
	if stdout, stderr, err := runCommand(fmt.Sprintf("%s --kubernetes-version=%s init",
		kubeSpawnPath, k8sStableVersion),
	); err != nil {
		t.Fatalf("error running kube-spawn init: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// set env variable KUBECONFIG to $GOPATH/src/github.com/kinvolk/kube-spawn/.kube-spawn/default/kubeconfig
	if err := os.Setenv("KUBECONFIG", utils.GetValidKubeConfig()); err != nil {
		t.Fatalf("error running setenv: %v\n", err)
	}
	checkK8sNodes(t)
}

func TestMainK8sStable(t *testing.T) {
	checkRequirements(t)
	initPath(t)
	initNode(t)

	testSetupK8sStable(t)
	testInitK8sStable(t)
}
