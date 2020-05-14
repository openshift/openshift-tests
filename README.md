# Extended Platform Tests

This repository holds the non-kubernetes, end-to-end tests that need to pass on a running
cluster before PRs merge and/or before we ship a release.
These tests are based on ginkgo and the github.com/kubernetes/kubernetes e2e test framework.

Prerequisites
-------------

* Git installed.
* Golang installed.
* Have the environment variable `KUBECONFIG` set pointing to your cluster.

### New Test Folder
If you create a new folder for your test case, please **add the path** to the [include.go file](https://github.com/openshift/openshift-tests/blob/master/test/extended/include.go).

## Compile the executable binary
The generated `extended-platform-tests` binary in the `cmd/extended-platform-tests/` folder.
If you want to compile the `openshift-tests` binary, please see the [origin](https://github.com/openshift/origin).

```console
$ mkdir -p ${GOPATH}/src/github.com/openshift/
$ cd ${GOPATH}/src/github.com/openshift/
$ git clone git@github.com:openshift/openshift-tests.git
$ make clean
$ make build
```

Run `./extended-platform-tests --help` to get started.

```console
This command verifies behavior of an OpenShift cluster by running remote tests against the cluster API that exercise functionality. In general these tests may be disruptive or require elevated privileges - see the descriptions of each test suite.

Usage:
   [command]

Available Commands:
  help        Help about any command
  run         Run a test suite
  run-monitor Continuously verify the cluster is functional
  run-test    Run a single test by name
  run-upgrade Run an upgrade suite

Flags:
  -h, --help   help for this command
```

## How to run

You can filter your test case by using `grep`. Such as, 
For example, to filter the [OLM test cases](https://github.com/openshift/openshift-tests/blob/master/test/extended/operators/olm.go#L21), you can run this command: 

```console
$ ./bin/extended-platform-tests run all --dry-run|grep "\[Feature:Platform\] OLM should"
I0410 15:33:38.465141    7508 test_context.go:419] Tolerating taints "node-role.kubernetes.io/master" when considering if nodes are ready
"[Feature:Platform] OLM should Implement packages API server and list packagemanifest info with namespace not NULL [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should [Serial] olm version should contain the source commit id [Suite:openshift/conformance/serial]"
"[Feature:Platform] OLM should be installed with catalogsources at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with clusterserviceversions at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with installplans at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with operatorgroups at version v1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with packagemanifests at version v1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should be installed with subscriptions at version v1alpha1 [Suite:openshift/conformance/parallel]"
"[Feature:Platform] OLM should have imagePullPolicy:IfNotPresent on thier deployments [Suite:openshift/conformance/parallel]"
```

You can save the above output to a file and run it:

```console
$ ./bin/extended-platform-tests run -f <your file path/name>
```

Or you can run it directly:

```console
$ ./bin/extended-platform-tests run all --dry-run | grep "\[Feature:Platform\] OLM should" | ./bin/extended-platform-tests run --junit-dir=./ -f -
```

### How to run a specific test case
It searches the test case title by RE(`Regular Expression`). So you need to specify the title string detailly.
For example, to run this test case: ["[Serial] olm version should contain the source commit id"](https://github.com/openshift/openshift-tests/blob/master/test/extended/operators/olm.go#L117), you can do it with 2 ways:

* You may filter the list and pass it back to the run command with the --file argument. You may also pipe a list of test names, one per line, on standard input by passing "-f -".

```console
$ ./bin/extended-platform-tests run all --dry-run|grep "\[Serial\] olm version should contain the source commit id"|./bin/extended-platform-tests run --junit-dir=./ -f -
```

* You can also run it as follows if you know which test suite it belongs to.

```console
$ ./bin/extended-platform-tests run openshift/conformance/serial --run "\[Serial\] olm version should contain the source commit id"
```

## How to generate bindata
If you have some new YAML files used in your code, you have to generate the bindata first.
Run `make update` to update the bindata. For example, you can see the bindata has been updated after running the `make update`. As follows: 
```console
$ git status
	modified:   test/extended/testdata/bindata.go
	new file:   test/extended/testdata/olm/etcd-subscription-manual.yaml
```

## Run ISV Operators test

```console
$ ./bin/extended-platform-tests run openshift/isv --dry-run | grep -E "<REGEX>" | ./bin/extended-platform-tests run -f -
```
