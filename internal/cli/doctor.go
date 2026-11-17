package cli

import (
	"context"
	"fmt"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
	"github.com/akkaraponph/itervyn/internal/providers/evaris"
	"github.com/akkaraponph/itervyn/internal/providers/invaris"
	"github.com/akkaraponph/itervyn/internal/verify"
	"io"
	"os"
)

type readinessCheck struct {
	Provider string `json:"provider"`
	Required bool   `json:"required"`
	Code     string `json:"code"`
	Ready    bool   `json:"ready"`
}
type readiness struct {
	SchemaVersion string            `json:"schemaVersion"`
	Ready         bool              `json:"ready"`
	Checks        []readinessCheck  `json:"checks"`
	Warnings      []verify.Feedback `json:"warnings"`
}

func doctor(ctx context.Context, c config.Resolved, jsonMode bool, stdout, stderr io.Writer) int {
	r := readiness{SchemaVersion: verify.DoctorSchema, Ready: true, Checks: []readinessCheck{}, Warnings: []verify.Feedback{}}
	add := func(name string, required bool, err error) {
		row := readinessCheck{Provider: name, Required: required, Ready: err == nil}
		if err != nil {
			row.Code = config.Code(err)
			if required {
				r.Ready = false
			} else {
				r.Warnings = append(r.Warnings, verify.Feedback{Provider: verify.Ptr(name), Code: row.Code, Message: "Optional provider is not ready."})
			}
		}
		r.Checks = append(r.Checks, row)
	}
	if ctx.Err() != nil {
		return diagnostic(stdout, stderr, jsonMode, config.Err(process.ContextReason(ctx), "readiness check interrupted"))
	}
	for _, p := range []struct {
		name, executable  string
		enabled, required bool
	}{{"invaris", c.Providers.Invaris.Executable, c.Providers.Invaris.Enabled, c.Providers.Invaris.Required}, {"evaris", c.Providers.Evaris.Executable, c.Providers.Evaris.Enabled, c.Providers.Evaris.Required}} {
		if !p.enabled {
			continue
		}
		_, err := process.Identify(p.executable, p.name)
		if err == nil {
			if p.name == "invaris" {
				var b []byte
				b, err = config.Read(c.ProjectRoot, c.Providers.Invaris.Harness, config.MaxConfig)
				if os.IsNotExist(err) {
					err = config.Err("HARNESS_MISSING", "root harness unavailable")
				}
				if err == nil {
					_, err = invaris.ParseHarness(b)
				}
			} else {
				err = evaris.Prerequisites(c)
			}
		}
		add(p.name, p.required, err)
	}
	// Only this owned permission probe is written. Providers run only
	// `--version` during identification; no guard or verification executes.
	err := config.CheckDescendants(c.ProjectRoot, c.Evidence.Directory, true)
	if err == nil {
		anchor, e := os.OpenRoot(c.ProjectRoot)
		if e != nil {
			err = e
		} else {
			err = anchor.MkdirAll(c.Evidence.Directory, 0700)
			_ = anchor.Close()
		}
	}
	if err == nil {
		f, e := os.CreateTemp(c.EvidenceDir, ".doctor-")
		if e != nil {
			err = e
		} else {
			err = f.Close()
			if e = os.Remove(f.Name()); err == nil {
				err = e
			}
		}
	}
	add("evidence", true, err)
	if jsonMode {
		err = emit(stdout, r)
	} else {
		for _, row := range r.Checks {
			fmt.Fprintf(stdout, "%s: ready=%t required=%t %s\n", row.Provider, row.Ready, row.Required, row.Code)
		}
		fmt.Fprintln(stdout, "Readiness does not prove verification passes.")
	}
	if err != nil {
		return 2
	}
	if !r.Ready {
		return 2
	}
	return 0
}
