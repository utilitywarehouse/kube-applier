# AGENTS.md

Guidance for coding agents working in this repository. Keep it short and correct;
update it when a convention changes.

## What this is

kube-applier (`github.com/utilitywarehouse/kube-applier`, Go 1.25) is a Kubernetes
controller that applies declarative manifests from a git repo to the cluster it
runs in. Entry point is `main.go`; the core packages are:

- `run/` — scheduler, runner, and the apply loop (most logic lives here)
- `git/` — repository polling and change detection
- `client/` — Kubernetes client wrappers
- `apis/` — the `Waybill` CRD types
- `kubectl/`, `kustomizeutil/`, `webserver/`, `metrics/`, `sysutil/`, `log/`

## Testing — use the Makefile, not `go test` directly

The `run` package tests use envtest, which needs kube control-plane binaries
(etcd/kube-apiserver). Running `go test ./run/...` directly fails with
`fork/exec /usr/local/kubebuilder/bin/etcd: no such file or directory` because
`KUBEBUILDER_ASSETS` is unset.

`make test` handles this: it installs `setup-envtest`, downloads the binaries
into `./kubebuilder-bindir` (gitignored), exports `KUBEBUILDER_ASSETS`, and runs
the whole suite with `-race` and coverage. This is the single target for
everything; CI runs it too.

Notes:
- A few specs need the `strongbox` binary on PATH (it decrypts fixtures via a git
  filter). They self-skip when it's missing, so you don't need strongbox
  installed to run the suite.
- First run needs network access to fetch the envtest assets; later runs reuse
  the cache in `kubebuilder-bindir`.
- envtest is pinned to `KUBEBUILDER_VERSION` (currently `1.30.x`) in the Makefile,
  independent of the runtime kubectl version.
- Pure unit tests that don't touch envtest (e.g. `TestWaybillsWithGitChanges`)
  can be run with plain `go test -run ...`, but when in doubt use `make test`.

## Codegen

CRD/RBAC manifests and deepcopy code are generated — after changing `apis/`:

- `make manifests` — regenerate CRDs (needs `controller-gen`, auto-installed)
- `make generate` — regenerate deepcopy code

## Build

- `go build ./...` for a quick compile check
- `make build` builds the Docker image

## Conventions

- Commit messages: plain imperative sentences, no conventional-commit prefixes
  (`feat:`/`fix:`/etc.) — matches this repo's history.
