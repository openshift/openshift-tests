# Operators Test Suite

This document describes how a developer can run/write a new extended test for
OpenShift Operator testing.


Prerequisites
-------------

* Compile both `oc` and `openshift-tests` in this repository (with `go build`)
* Have the environment variable `KUBECONFIG` set pointing to your cluster.


Running Tests
-------------

To run the test suite

```console
$ extended-platform-tests run openshift/isv
```

See the description on the test for more info about what prerequites may exist for the test.

To run a subset of tests using a regexp, run:

```console
$ extended-platform-tests run openshift/isv --dry-run | grep -E "<REGEX>" | extended-platform-tests run -f -
```

