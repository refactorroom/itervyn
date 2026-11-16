# Deterministic Go fixture

This authored marker guard is an offline protocol example. It reads `app.go`,
returns structured inspection JSON, and exits 1 on FORBIDDEN. SAFE passes. REFUSE
deliberately emits malformed evidence so native Invaris refuses verification.
It does not test general Go semantics or measure an AI model.

From Itervyn's repository root, build the helper into a copied disposable example:

```sh
example_dir=$(mktemp -d)
cp -R examples/go-project/. "$example_dir/"
go build -trimpath -o "$example_dir/guard-bin" ./examples/go-project/guard
itervyn verify --config "$example_dir/itervyn.json" --format json
```

Install approved Invaris v0.3.0 first or put its absolute executable path in the
copied itervyn.json. Change SAFE to FORBIDDEN in the copied app.go, observe exit 1
and FAIL, restore SAFE, and observe a new PASS run. Evaris remains disabled until
an actual native imported-run context is configured. Private output belongs in
the copied example's `.itervyn` directory, not in a commit.
