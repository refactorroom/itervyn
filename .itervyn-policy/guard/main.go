// Command guard reads imports from the candidate without building its code.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const policy = "verify-no-provider-imports"
const adapters = "github.com/akkaraponph/itervyn/internal/providers"

type finding struct {
	PolicyID string `json:"policyId"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
}

type inspection struct {
	Version   string    `json:"version"`
	PolicyID  string    `json:"policyId"`
	Inspected int       `json:"inspected"`
	Findings  []finding `json:"findings"`
}

func inspect() (inspection, error) {
	result := inspection{Version: "invaris.inspection/v1alpha1", PolicyID: policy, Findings: []finding{}}
	for _, path := range []string{"internal", "internal/verify"} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() {
			return result, errors.New("verification source directory unavailable")
		}
	}
	err := filepath.WalkDir("internal/verify", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("linked verification source")
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("non-regular verification source")
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		result.Inspected++
		for _, i := range file.Imports {
			name, err := strconv.Unquote(i.Path.Value)
			if err != nil {
				return err
			}
			if name == adapters || strings.HasPrefix(name, adapters+"/") {
				result.Findings = append(result.Findings, finding{policy, filepath.ToSlash(path), positions.PositionFor(i.Pos(), false).Line})
			}
		}
		return nil
	})
	return result, err
}

func main() {
	result, err := inspect()
	if err != nil || result.Inspected == 0 {
		fmt.Fprintln(os.Stderr, "Verification imports could not be inspected.")
		os.Exit(2)
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		os.Exit(2)
	}
	if len(result.Findings) != 0 {
		os.Exit(1)
	}
}
