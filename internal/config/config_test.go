package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const minimal = `{"schemaVersion":"itervyn.config/v1alpha1","providers":{"invaris":{"enabled":true,"required":true}}}`

func file(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "itervyn.json")
	if err := os.WriteFile(p, []byte(s), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestDefaultsAndResolution(t *testing.T) {
	p := file(t, minimal)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Execution.TimeoutSeconds != 300 || c.Providers.Invaris.Action != "check" || c.Providers.Evaris.Enabled || r.ConfigSHA256 != Hash([]byte(minimal)) {
		t.Fatalf("wrong defaults: %+v", r)
	}
	starter, _ := json.Marshal(Starter())
	if _, err := Load(file(t, string(starter))); err != nil {
		t.Fatal("starter invalid:", err)
	}
}
func TestRejectConfig(t *testing.T) {
	cases := []string{
		`{}`, `null`, `[]`, minimal + ` {}`, strings.Replace(minimal, `"schemaVersion":`, `"schemaVersion":"x","schemaVersion":`, 1),
		strings.Replace(minimal, `"enabled":true`, `"enabled":true,"enabl\u0065d":false`, 1),
		strings.Replace(minimal, `"enabled":true`, `"Enabled":true`, 1), strings.Replace(minimal, `"enabled":true`, `"enabled":null`, 1),
		strings.Replace(minimal, `"enabled":true,`, ``, 1), strings.Replace(minimal, `"required":true`, `"required":false`, 1),
		strings.Replace(minimal, `"enabled":true`, `"enabled":false`, 1), strings.Replace(minimal, `"invaris":{"enabled":true,"required":true}`, `"invaris":{}`, 1),
	}
	for _, extra := range []string{
		`"project":null`, `"project":{"root":null}`, `"project":{"root":""}`, `"project":{"root":"../escape"}`,
		`"evidence":{"directory":"../escape"}`, `"evidence":{"directory":"/absolute"}`, `"evidence":{"directory":""}`,
		`"execution":null`, `"execution":{"timeoutSeconds":null}`, `"execution":{"timeoutSeconds":0}`, `"execution":{"timeoutSeconds":3601}`,
		`"execution":{"timeoutSeconds":1.5}`, `"execution":{"timeoutSeconds":999999999999999999999}`, `"execution":{"stdoutLimitBytes":4194305}`,
		`"execution":{"stderrLimitBytes":0}`, `"execution":{"providerTimeoutSeconds":-1}`, `"execution":{"environmentAllowlist":null}`,
		`"execution":{"environmentAllowlist":["PATH"]}`, `"execution":{"environmentAllowlist":["A","A"]}`, `"execution":{"environmentAllowlist":[""]}`,
		`"unknown":true`,
	} {
		cases = append(cases, strings.Replace(minimal, `{"schemaVersion"`, `{`+extra+`,"schemaVersion"`, 1))
	}
	for _, extra := range []string{`"action":"run"`, `"harness":"sub/invaris.json"`, `"harness":".."`, `"executable":"./invaris"`, `"executable":""`, `"store":"x"`} {
		cases = append(cases, strings.Replace(minimal, `"enabled":true`, `"enabled":true,`+extra, 1))
	}
	for _, extra := range []string{`{"enabled":true,"required":true}`, `{"enabled":true,"required":false,"store":"/tmp/store","runId":"latest"}`, `{"enabled":false,"required":false,"store":""}`, `null`} {
		cases = append(cases, strings.Replace(minimal, `"invaris":`, `"evaris":`+extra+`,"invaris":`, 1))
	}
	cases = append(cases, string([]byte{0xff}))
	for n, s := range cases {
		if _, err := Load(file(t, s)); err == nil {
			t.Fatalf("case %d accepted: %s", n, s)
		}
	}
}
func TestSafeFiles(t *testing.T) {
	p := file(t, minimal)
	c, _ := Load(p)
	root := filepath.Dir(p)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".itervyn")); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(c, p); err == nil {
		t.Fatal("evidence symlink accepted")
	}
	if err := os.Remove(filepath.Join(root, ".itervyn")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".itervyn"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(c, p); err == nil {
		t.Fatal("non-directory ancestor accepted")
	}
	link := filepath.Join(root, "linked.json")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("linked configuration accepted")
	}
	if _, err := Read(root, "../outside", 100); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root, "fifo", 100); err == nil {
		t.Fatal("special file accepted")
	}
	if err := os.WriteFile(p, []byte(strings.Replace(minimal, `"required":true`, `"required":false`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(c, p); err == nil {
		t.Fatal("changed config accepted")
	}
}
