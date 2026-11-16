# Contributing

Use Go 1.27.4 or newer on Linux/macOS. Runtime dependencies are standard-library
only. Preserve CLI/config/result contracts and keep provider algorithms upstream.
Format Go changes and add tests that reject plausible incorrect behavior.

```sh
gofmt -l cmd internal examples
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go build ./...
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go vet ./...
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./...
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test -race ./...
```

gofmt must produce no paths. Select an installed Go 1.27.4+ binary first; a
toolchain-launcher download may need checksum access before the offline phase.

Real-provider tests are separate from fake-provider contract handling. Build clean
Invaris d770885fd33012d9ff368b25611f8b327d348b6e and Evaris
e12566ba22fa2d63fe3ecb68ed61bf3dbd38b065 outside their modules' source trees:

```sh
export ITERVYN_TEST_INVARIS=/absolute/approved/invaris
export ITERVYN_TEST_EVARIS=/absolute/approved/evaris
GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off go test ./internal/acceptance -count=1 -v
```

Unset variables skip the corresponding explicitly named real-provider tests
locally. CI must provision both or fail; skips do not establish compatibility.
Optional `ITERVYN_EVIDENCE_DIR` retains private copies of synthetic run records;
point it inside an ignored, private directory. Test fixtures are authored
synthetic controls, not measurements of real agent sessions.

Do not commit `.panda-suite`, `.itervyn`, private evaluations, transcripts, binaries
or secrets. Preserve schema provenance and Apache-2.0 notices when incorporating
upstream material. No external implementation or Agent Skills are vendored.
Run the [release checks](docs/release.md) before distribution. Changes to public
contracts require documentation, schema and compatibility-test updates.

For independent JSON Schema conformance checks (development only):

```sh
python3 -m venv .panda-suite/schema-venv
.panda-suite/schema-venv/bin/pip install -r scripts/requirements-schema.txt
.panda-suite/schema-venv/bin/python scripts/validate-schemas.py --evidence .panda-suite/evidence
```

Dependency provisioning may access the network. The standalone CLI has no Python
or third-party Go runtime dependency. This check validates actual result/config
pairs and rejects deliberately invalid controls.
