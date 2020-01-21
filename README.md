# Extended Platform Tests

This repository holds the non-kubernetes end-to-end tests that need to pass on a running
cluster before PRs merge and/or before we ship a release.
These tests are based on ginkgo and the github.com/kubernetes/kubernetes e2e test framework.

Run `./extended-platform-tests run-tests --help` or `./extended-platform-tests run-test --help` to get started.

## How to run

`./extended-platform-tests` matches the existing signature and arguments for `openshift-tests` from github.com/openshift/origin.

