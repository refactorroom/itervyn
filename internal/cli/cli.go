// Package cli implements user-facing arguments and rendering; verification policy
// belongs to verify.Service.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/verify"
	"io"
	"path/filepath"
	"strings"
)

const usage = `usage: itervyn <init|doctor|verify|inspect|version> [flags]
  init, doctor, verify: [--config PATH] [--format text|json]
  inspect RUN_ID: [--config PATH] [--format text|json]
  version: [--format text|json]
  --help, -h: show help
`

type arguments struct {
	command, config, format, id string
	help                        bool
}

func parse(args []string) (arguments, error) {
	a := arguments{config: "itervyn.json", format: "text"}
	if len(args) == 0 {
		return a, config.Err("INVALID_ARGUMENT", "expected a command")
	}
	a.command = args[0]
	if a.command == "--help" || a.command == "-h" {
		if len(args) == 1 {
			a.help = true
			return a, nil
		}
		return a, config.Err("INVALID_ARGUMENT", "extra help arguments")
	}
	if !strings.Contains("|init|doctor|verify|inspect|version|", "|"+a.command+"|") {
		return a, config.Err("INVALID_ARGUMENT", "unknown command")
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		s := args[i]
		if s == "--help" || s == "-h" {
			a.help = true
			continue
		}
		if strings.HasPrefix(s, "--") {
			parts := strings.SplitN(s, "=", 2)
			key := parts[0]
			if seen[key] || key != "--config" && key != "--format" || a.command == "version" && key == "--config" {
				return a, config.Err("INVALID_ARGUMENT", "unknown or repeated flag")
			}
			seen[key] = true
			v := ""
			if len(parts) == 2 {
				v = parts[1]
			} else {
				if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
					return a, config.Err("INVALID_ARGUMENT", "flag requires a value")
				}
				i++
				v = args[i]
			}
			if v == "" {
				return a, config.Err("INVALID_ARGUMENT", "empty flag value")
			}
			if key == "--config" {
				a.config = v
			} else {
				a.format = v
			}
			continue
		}
		if a.command != "inspect" || a.id != "" {
			return a, config.Err("INVALID_ARGUMENT", "unexpected positional argument")
		}
		a.id = s
	}
	if a.format != "text" && a.format != "json" {
		return a, config.Err("INVALID_ARGUMENT", "unsupported output format")
	}
	if a.command == "inspect" && a.id == "" && !a.help {
		return a, config.Err("INVALID_ARGUMENT", "inspect needs a run ID")
	}
	return a, nil
}
func wantsJSON(args []string) bool {
	for i, s := range args {
		if s == "--format=json" || s == "--format" && i+1 < len(args) && args[i+1] == "json" {
			return true
		}
	}
	return false
}
func emit(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func diagnostic(stdout, stderr io.Writer, jsonMode bool, err error) int {
	code := config.Code(err)
	message := "Operation failed; inspect configuration and private evidence."
	if f, ok := err.(*config.Failure); ok {
		message = f.Detail
	}
	// Only fixed application messages are rendered; no native stdout/stderr.
	fmt.Fprintf(stderr, "%s: %s\n", code, message)
	if jsonMode {
		_ = emit(stdout, verify.Diagnostic{SchemaVersion: verify.DiagnosticSchema, Code: code, Message: message, Status: verify.Error})
	}
	return 2
}
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, service verify.Service) int {
	a, err := parse(args)
	if err != nil {
		return diagnostic(stdout, stderr, wantsJSON(args), err)
	}
	if a.help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	jsonMode := a.format == "json"
	if a.command == "version" {
		if jsonMode {
			err = emit(stdout, struct {
				Version           string   `json:"version"`
				ConfigSchemas     []string `json:"configSchemas"`
				ResultSchemas     []string `json:"resultSchemas"`
				DiagnosticSchemas []string `json:"diagnosticSchemas"`
				DoctorSchemas     []string `json:"doctorSchemas"`
			}{verify.Version, []string{config.Schema}, []string{verify.ResultSchema}, []string{verify.DiagnosticSchema}, []string{verify.DoctorSchema}})
		} else {
			_, err = fmt.Fprintln(stdout, "itervyn", verify.Version)
		}
		if err != nil {
			return 2
		}
		return 0
	}
	if a.command == "init" {
		return initConfig(a.config, jsonMode, stdout, stderr)
	}
	c, err := config.Load(a.config)
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	r, err := config.Resolve(c, a.config)
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	if a.command == "doctor" {
		return doctor(ctx, r, jsonMode, stdout, stderr)
	}
	var result verify.Result
	if a.command == "inspect" {
		if service.Store == nil {
			return diagnostic(stdout, stderr, jsonMode, config.Err("INTERNAL_ERROR", "missing evidence store"))
		}
		result, err = service.Store.Inspect(ctx, r.EvidenceDir, a.id)
		if err != nil {
			return diagnostic(stdout, stderr, jsonMode, err)
		}
	} else {
		result, err = service.Verify(ctx, verify.Request{Config: r})
	}
	if jsonMode {
		if e := emit(stdout, result); e != nil {
			return 2
		}
	} else {
		fmt.Fprintf(stdout, "%s — %s\nrun: %s\n", result.Status, result.NextAction, result.RunID)
		for _, p := range result.Providers {
			fmt.Fprintf(stdout, "%s: %s (required=%t, scope=%s)\n", p.Name, p.Status, p.Required, p.Scope.Kind)
		}
		for _, f := range append(append([]verify.Feedback{}, result.Feedback...), result.Warnings...) {
			fmt.Fprintf(stdout, "%s: %s\n", f.Code, f.Message)
		}
		if result.EvidencePersisted {
			fmt.Fprintln(stdout, "evidence:", filepath.Join(r.EvidenceDir, result.RunID))
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "IO_ERROR: verification record was not committed")
		return 2
	}
	return verify.Exit(result.Status)
}
