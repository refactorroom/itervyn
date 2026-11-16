package cli

import (
	"encoding/json"
	"fmt"
	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/verify"
	"io"
	"os"
	"path/filepath"
)

func initConfig(path string, jsonMode bool, stdout, stderr io.Writer) int {
	if !config.SafeLexical(path) {
		return diagnostic(stdout, stderr, jsonMode, config.Err("PATH_UNSAFE", "unsafe configuration destination"))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	r, err := os.OpenRoot(parent)
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	defer r.Close()
	b, err := json.MarshalIndent(config.Starter(), "", "  ")
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	b = append(b, '\n')
	f, err := r.OpenFile(filepath.Base(abs), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, config.Err("IO_ERROR", "configuration destination exists or cannot be created"))
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return diagnostic(stdout, stderr, jsonMode, err)
	}
	warnings := []verify.Feedback{}
	if _, e := config.Read(parent, "invaris.json", config.MaxConfig); e != nil {
		warnings = append(warnings, verify.Feedback{Code: "HARNESS_MISSING", Message: "Create a supported root invaris.json harness, then run itervyn doctor."})
	}
	if jsonMode {
		err = emit(stdout, struct {
			Created    bool              `json:"created"`
			ConfigPath string            `json:"configPath"`
			Warnings   []verify.Feedback `json:"warnings"`
		}{true, abs, warnings})
	} else {
		fmt.Fprintln(stdout, "Created configuration. Run itervyn doctor to check prerequisites.")
		for _, w := range warnings {
			fmt.Fprintln(stdout, w.Code+": "+w.Message)
		}
	}
	if err != nil {
		return 2
	}
	return 0
}
