package cli

import (
	"bytes"
	"context"
	"fmt"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/process"
	"github.com/akkaraponph/itervyn/internal/providers/evaris"
	"github.com/akkaraponph/itervyn/internal/providers/invaris"
	"github.com/akkaraponph/itervyn/internal/verify"
	"io"
	"os"
	"path/filepath"
)

type readinessCheck struct {
	Provider string `json:"provider"`
	Required bool   `json:"required"`
	Code     string `json:"code"`
	Ready    bool   `json:"ready"`
	Version  string `json:"version,omitempty"`
}
type readiness struct {
	SchemaVersion string            `json:"schemaVersion"`
	Ready         bool              `json:"ready"`
	Checks        []readinessCheck  `json:"checks"`
	Warnings      []verify.Feedback `json:"warnings"`
}

func doctor(ctx context.Context, c config.Resolved, jsonMode bool, stdout, stderr io.Writer) int {
	r := readiness{SchemaVersion: verify.DoctorSchema, Ready: true, Checks: []readinessCheck{}, Warnings: []verify.Feedback{}}
	add := func(name string, required bool, version string, err error) {
		row := readinessCheck{Provider: name, Required: required, Ready: err == nil, Version: version}
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
		identity, err := process.Identify(p.executable, p.name)
		version := ""
		if identity.Module == "github.com/akkaraponph/"+p.name && process.ReleaseLine(identity.Version) != "" {
			version = identity.Version
		}
		if err == nil {
			if p.name == "invaris" {
				var b []byte
				b, err = c.ReadHarness()
				if os.IsNotExist(err) {
					err = config.Err("HARNESS_MISSING", "root harness unavailable")
				}
				if err == nil {
					_, err = invaris.ParseHarness(b)
				}
				if err == nil && c.SchemaVersion == config.TrustedSchema {
					staged, readErr := c.ReadRuntimeHarness()
					if readErr != nil || !bytes.Equal(b, staged) {
						err = config.Err("EVIDENCE_MISMATCH", "staged harness differs from trusted policy")
					}
				}
			} else {
				err = evaris.Prerequisites(c)
			}
		}
		add(p.name, p.required, version, err)
	}
	// Only this owned permission probe is written. Providers run only
	// `--version` during identification; no guard or verification executes.
	err := config.CheckAbsolute(c.EvidenceDir, true)
	if err == nil {
		anchor := string(filepath.Separator)
		root, openErr := os.OpenRoot(anchor)
		if openErr != nil {
			err = openErr
		} else {
			defer root.Close()
			rel, _ := filepath.Rel(anchor, c.EvidenceDir)
			err = root.MkdirAll(rel, 0700)
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
	add("evidence", true, "", err)
	if jsonMode {
		err = emit(stdout, r)
	} else {
		for _, row := range r.Checks {
			fmt.Fprintf(stdout, "%s: ready=%t required=%t", row.Provider, row.Ready, row.Required)
			if row.Version != "" {
				fmt.Fprintf(stdout, " version=%s", row.Version)
			}
			if row.Code != "" {
				fmt.Fprintf(stdout, " %s", row.Code)
			}
			fmt.Fprintln(stdout)
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
