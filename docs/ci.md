# CI and local checks

CI runs without Azure credentials and uses read-only GitHub token permissions.
All third-party GitHub Actions and container base images are pinned. The
container job builds and scans the image locally; it never pushes an image.

Run the primary checks from the repository root:

```sh
gofmt -w $(git ls-files '*.go')
go mod tidy
go vet ./...
make test-risk-coverage
python3 scripts/check-compatibility.py
bash scripts/render-manifests.sh
```

Run both real-client ExternalDNS contract suites:

```sh
cd tests/contract
go test -mod=readonly -modfile=v0.23.0.mod -race -count=1 ./...
go test -mod=readonly -race -count=1 ./...
```

Run the deployment script renderers with fixture values (the `--render` mode
never contacts Azure or a cluster):

```sh
bash scripts/render-manifests.sh
```

The `manifests` CI job checks YAML schemas and applies the CRD and every checked
in or rendered resource to disposable Kubernetes API servers for 1.35, 1.36,
and 1.37. Contract tests use upstream's exact ExternalDNS tags. `make
test-risk-coverage` uses function-level thresholds in `coverage-policy.json`;
the repository-wide average is informational and cannot mask a risk-path
regression.

The required pull-request check is **CI required**. Fix failing checks rather
than skipping them. Security and license exceptions must follow the narrow,
reviewed, expiring process in [compatibility policy](compatibility.md).
