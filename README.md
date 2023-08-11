# kube-applier

Forked from: https://github.com/box/kube-applier

kube-applier is Kubernetes deployment tool strongly following
[gitOps](https://www.weave.works/blog/gitops-operations-by-pull-request)
principals. It enables continuous deployment of Kubernetes objects by applying
declarative configuration files from a Git repository to a Kubernetes cluster.

kube-applier runs as a Deployment in your cluster and watches the [Git
repo](#mounting-the-git-repository) to ensure that the cluster objects are
up-to-date with their associated spec files (JSON or YAML) in the repo.

Configuration is done through the `kube-applier.io/Waybill` CRD. Each namespace
in a cluster defines a Waybill CRD which defines the source of truth for the
namespace inside the repository.

Whenever a new commit to the repo occurs, or at a [specified
interval](#run-interval), kube-applier performs a "run", issuing [kubectl
apply](https://kubernetes.io/docs/user-guide/kubectl/v1.6/#apply) commands at
namespace level.

kube-applier serves a [status page](#status-ui) and provides
[metrics](#metrics) for monitoring.

## Usage

Please refer to the command line `-help` for information on the various
configuration options. Each flag corresponds to an environment variable (eg.:
`-repo-path` is `REPO_PATH`).

They are all optional values expect for `-repo-remote` which is required for
kube-applier to start.

### Waybill CRD

kube-applier behaviour is controlled through the Waybill CRD. Refer to the code
or CRD yaml definition for details, an example with the default values is shown
below:

```yaml
apiVersion: kube-applier.io/v1alpha1
kind: Waybill
metadata:
  name: main
spec:
  autoApply: true
  delegateServiceAccountSecretRef: kube-applier-delegate
  dryRun: false
  gitSSHSecretRef:
    name: ""
    namespace: ""
  prune: true
  pruneClusterResources: false
  pruneBlacklist: []
  repositoryPath: <namespace-name>
  runInterval: 3600
  runTimeout: 900
  serverSideApply: false
  strongboxKeyringSecretRef:
    name: ""
    namespace: ""
```

See the documentation on the Waybill CRD
[spec](https://godoc.org/github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1#WaybillSpec)
for more details.

#### Delegate ServiceAccount

To avoid leaking access from kube-applier to client namespaces, the concept of a
delegate ServiceAccount is introduced. When applying a Waybill, kube-applier
will use the credentials defined in the Secret referenced by
`delegateServiceAccountSecretRef`. This is a ServiceAccount in the same
namespace as the Waybill itself and should typically be given admin access to
the namespace. See the [client base](./manifests/base/client) for an example of
how to set this up.

This secret should be version controlled but will have to be manually applied
at first in order to bootstrap the kube-applier integration in a namespace.

#### Integration with `strongbox`

[strongbox](https://github.com/uw-labs/strongbox) is an encryption tool, geared
towards git repositories and working as a git filter.

If `strongboxKeyringSecretRef` is defined in the Waybill spec (it is an object
that contains the attributes `name` and `namespace`), it should reference a
Secret resource which contains a key named `.strongbox_keyring` with its value
being a valid strongbox keyring file. That keyring is subsequently used when
applying the Waybill, allowing for decryption of files under the
`repositoryPath`. If the attribute `namespace` for `strongboxKeyringSecretRef`
is not specified then it defaults to the same namespace as the Waybill itself.

This secret should be readable by the ServiceAccount of kube-applier. If
deployed using the provided kustomize bases, kube-applier's ServiceAccount will
have read access to secrets named `"kube-applier-strongbox-keyring"` by default.
Alternatively, if you need to use a different name, you will need to create a
Role and a RoleBinding to give `"get"` permission to it.

If you need to deploy a shared strongbox keyring to use in multiple namespaces,
the Secret should have an annotation called
`"kube-applier.io/allowed-namespaces"` which contains a comma-seperated list of
all the namespaces that are allowed to use it.

For example, the following secret can be used by namespaces "ns-a", "ns-b" and
"ns-c":

```
kind: Secret
apiVersion: v1
metadata:
  name: kube-applier-strongbox-keyring
  namespace: ns-a
  annotations:
    kube-applier.io/allowed-namespaces: "ns-b, ns-c"
stringData:
  .strongbox_keyring: |-
    keyentries:
    - description: mykey
      key-id: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
      key: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
```

Each item in the list of allowed namespaces supports [shell pattern
matching](https://golang.org/pkg/path/#Match), meaning you can grant access to
namespaces based on naming conventions:

```
kube-applier.io/allowed-namespaces: "team-a-*, team-b-monitoring"
```

The secret containing the strongbox keyring should itself be version controlled
to prevent kube-applier from pruning it. However, since it is a secret itself
and would need be encrypted as well in git, it must be created manually the
first time (or after any changes to its contents).

#### Custom SSH Keys

You can specify custom SSH keys to be used for fetching remote `kustomize` bases
from private repositories. In order to do that, you will need to specify
`gitSSHSecretRef`: it should reference a `Secret` that contains one or more SSH
keys that provide access to the private repositories that contain these bases.

In this `Secret`, items with a name that begins with `"key_*"` are treated as
SSH keys to be used with `kustomize`. Additionally this `Secret` can optionally
define a value for `"known_hosts"`. If ommitted, `git` will use `ssh` with
`StrictHostKeyChecking` disabled.

To use an SSH key for Kustomize bases, the bases should be defined with the
`ssh://` scheme in `kustomization.yaml` and have a `# kube-applier: key_foobar`
comment above it. For example:

```
bases:
  # https scheme (default if omitted), any SSH keys defined are ignored
  - github.com/utilitywarehouse/kube-applier//testdata/bases/simple-deployment?ref=master
  # ssh scheme requires a valid SSH key to be defined
  # kube-applier: key_a
  - ssh://github.com/utilitywarehouse/kube-applier//testdata/bases/simple-deployment?ref=master
```

This `Secret` can be shared in the same way that a strongbox keyring can be
shared, as shown in the example below:

```
kind: Secret
apiVersion: v1
metadata:
  name: kube-applier-git-ssh
  namespace: ns-a
  annotations:
    kube-applier.io/allowed-namespaces: "ns-b, ns-c"
stringData:
  key_a: |-
    -----BEGIN OPENSSH PRIVATE KEY-----
    AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
    -----END OPENSSH PRIVATE KEY-----
  key_b: |-
    -----BEGIN OPENSSH PRIVATE KEY-----
    AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
    -----END OPENSSH PRIVATE KEY-----
  known_hosts: |-
    github.com ssh-rsa AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==
```

### Resource pruning

Resource pruning is enabled by default and controlled by the `prune` attribute
of the Waybill spec. This means that if a file is removed from the git
repository, the resources defined in it will be pruned in the next run.

If you want kube applier to prune cluster resources, you can set
`pruneClusterResources` to `true`. Take care when using this feature as it will
remove all cluster resources that have been created by kubectl and don't exist
in the current namespace directory. Therefore, only use this feature if all
of your cluster resources are defined under one directory.

Specific resource types can be exempted from pruning by adding them to the
`pruneBlacklist` attribute:

```
  pruneBlacklist:
    - core/v1/ConfigMap
    - core/v1/Namespace
```

To blacklist resources for all namespaces, use the `PRUNE_BLACKLIST` environment
variable:

```
env:
  - name: PRUNE_BLACKLIST
    value: "core/v1/ConfigMap,core/v1/Namespace"
```

The resource `apps/v1/ControllerRevision` is always exempted from pruning,
regardless of the blacklist. This is because Kubernetes copies the
`kubectl.kubernetes.io/last-applied-configuration` annotation to controller
revisions from the corresponding StatefulSet, Deployment or Daemonset. This would
result in kube-applier pruning revisions that it shouldn't be managing if it
wasn't blacklisted.

It's important to note that kube-applier uses a
[`SelfSubjectRulesReview`](https://pkg.go.dev/k8s.io/api/authorization/v1#SelfSubjectRulesReview)
to establish which resources it has permissions to prune. This may not work
reliably if you aren't using the RBAC
[authorization module](https://kubernetes.io/docs/reference/access-authn-authz/authorization/#authorization-modules).
In which case, the prune allowlist may be empty or incomplete.

## Deploying

Included is a Kustomize (https://kustomize.io/) base you can reference in your
namespace:

```
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
- github.com/utilitywarehouse/kube-applier//manifests/base/server?ref=<version>
```

and patch as per example: [manifests/example/](manifests/example/)

Note that the example also includes the client base, for kube-applier to manage
its own namespace.

Please note that if you enable kustomize for your namespace and you've enabled
pruning in kube-applier, _all_ your resources need to be listed in your
`kustomization.yaml` under `resources`. If you don't do this kube-applier will
assume they have been removed and start pruning.

Additionally, you will need to deploy the following kustomize base that includes
cluster-level resources (also see this [section](#resource-pruning) if you are
using the `pruneClusterResources` attribute):

```
github.com/utilitywarehouse/kube-applier//manifests/base/cluster?ref=<version>
```

## Monitoring

### Status UI

kube-applier serves an HTML status page on which displays information about the
Waybills managed by it and their most recent apply runs, including:

- Apply run type (reason)
- Start times and latency
- Most recent commit
- Apply command, output and errors

The HTML template for the status page lives in `templates/status.html`, and
`static/` holds additional assets.

### Metrics

kube-applier uses [Prometheus](https://github.com/prometheus/client_golang) for
metrics. Metrics are hosted on the webserver at `/__/metrics`. In addition to
the Prometheus default metrics, the following custom metrics are included:

- **kube_applier_run_latency_seconds** - A
  [Histogram](https://godoc.org/github.com/prometheus/client_golang/prometheus#Histogram)
  that keeps track of the durations of each apply run, labelled with the
  namespace name, a success and a dry run boolean.

- **kube_applier_namespace_apply_count** - A
  [Counter](https://godoc.org/github.com/prometheus/client_golang/prometheus#Counter)
  for each namespace that has had an apply attempt over the lifetime of the
  container, incremented with each apply attempt and labelled by the namespace
  and the result of the attempt.

- **kube_applier_result_summary** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  for each resource, labelled with the namespace, action, status and type of
  object applied.

- **kube_applier_kubectl_exit_code_count** - A
  [Counter](https://godoc.org/github.com/prometheus/client_golang/prometheus#Counter)
  for each exit code returned by executions of `kubectl`, labelled with the
  namespace and exit code.

- **kube_applier_last_run_timestamp_seconds** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  that reports the last time a run finished, expressed in seconds since the Unix
  Epoch and labelled with the namespace name.

- **kube_applier_last_run_success** -  A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  that reports the results of the last run labelled with the namespace name.

- **kube_applier_run_queue** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  that reports the number of runs that are currently queued, labelled with the
  namespace name and the run type.

- **kube_applier_run_queue_failures** - A
  [Counter](https://godoc.org/github.com/prometheus/client_golang/prometheus#Counter)
  that observes the number of times a run failed to queue properly, labelled
  with the namespace name and the run type.

- **kube_applier_waybill_spec_auto_apply** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  that captures the value of autoApply in the Waybill spec, labelled with the
  namespace name.

- **kube_applier_waybill_spec_dry_run** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  that captures the value of dryRun in the Waybill spec, labelled with the
  namespace name.

- **kube_applier_waybill_spec_run_interval** - A
  [Gauge](https://godoc.org/github.com/prometheus/client_golang/prometheus#Gauge)
  that captures the value of runInterval in the Waybill spec, labelled with
  the namespace name.

The Prometheus [HTTP API](https://prometheus.io/docs/querying/api/) (also see
the [Go
library](https://github.com/prometheus/client_golang/tree/master/api/prometheus))
can be used for querying the metrics server.

## Running locally

```
# manifests git repository
export LOCAL_REPO_PATH="${HOME}/dev/work/kubernetes-manifests"

# directory within the manifests repository that contains namespace directories
export CLUSTER_DIR="exp-1-aws"

export DIFF_URL_FORMAT="https://github.com/utilitywarehouse/kubernetes-manifests/commit/%s"

# export values for any other options that are configured through environment
# variables (the rest have default values and are optional) eg.:
export DRY_RUN="true"
export LOG_LEVEL="info"
```

```
make build
make run
```

Note that `make run` will mount and use `${HOME}/.kube` for configuration, so
ensure that your config files are using your intended context.

## Running tests

Tests are written primarily using the `envtest` package of the
[controller-runtime](https://godoc.org/github.com/kubernetes-sigs/controller-runtime/)
project. Run `make test` to pull the required binary assets and run the tests.
Once the assets are present in `./testbin` you can invoke `go test` manually
if you need a particular set of flags, but first you need to point envtest to
the binaries: `export KUBEBUILDER_ASSETS=$PWD/testbin/bin`.

If you are writing tests, you might want to take a look at the
[tutorial](https://book.kubebuilder.io/cronjob-tutorial/writing-tests.html),
as well as the [`ginkgo`](http://onsi.github.io/ginkgo/) and
[`gomega`](https://onsi.github.io/gomega/) documentation.

## Releasing

Please use the make target "release" with "VERSION" argument to update all
references and clean up after, example:

```
$ make release VERSION=v3.3.3-rc.3
```

## Copyright and License

Copyright 2016 Box, Inc. All rights reserved.

Copyright (c) 2017-2023 Utility Warehouse Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
