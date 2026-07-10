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
- `kubectl/`, `kustomizeutil/`, `webserver/`, `metrics/`, `clock/`, `envtestassets/`, `log/`

## Testing

`go test -race -count=1 -cover ./...` works standalone — no Make target or
env vars required. The `envtestassets` package (called from `TestMain` in the
`run`, `client` and `webserver` suites) lazily fetches etcd/kube-apiserver
binaries via `setup-envtest` when `KUBEBUILDER_ASSETS` is unset, caching them
in `./kubebuilder-bindir` (gitignored). `make test` still works as a
convenience wrapper.

Notes:
- A few specs need the `strongbox` binary on PATH (it decrypts fixtures via a
  git filter). They self-skip when it's missing, so you don't need strongbox
  installed to run the suite.
- First run needs network access to fetch the envtest assets; later runs reuse
  the cache in `kubebuilder-bindir`.
- envtest is pinned to `1.30.x` (see `envtestassets.Version`), independent of
  the runtime kubectl version.

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
