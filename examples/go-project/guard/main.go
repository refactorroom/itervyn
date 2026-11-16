// This authored example guard demonstrates structured evidence, not a general
// Go correctness check. It is external to Itervyn's verification implementation.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	path := "app.go"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fixture source unavailable")
		os.Exit(2)
	}
	if bytes.Contains(b, []byte("REFUSE")) {
		fmt.Print("deliberately malformed fixture evidence")
		return
	}
	type finding struct {
		PolicyID string `json:"policyId"`
		Path     string `json:"path"`
	}
	f := []finding{}
	if bytes.Contains(b, []byte("FORBIDDEN")) {
		f = append(f, finding{"no-forbidden-marker", path})
	}
	_ = json.NewEncoder(os.Stdout).Encode(struct {
		Version   string    `json:"version"`
		PolicyID  string    `json:"policyId"`
		Inspected int       `json:"inspected"`
		Findings  []finding `json:"findings"`
	}{"invaris.inspection/v1alpha1", "no-forbidden-marker", 1, f})
	if len(f) > 0 {
		os.Exit(1)
	}
}
