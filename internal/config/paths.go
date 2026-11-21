package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SafeLexical(p string) bool {
	if p == "" || strings.ContainsAny(p, "\x00\\") {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(p), "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
func SafeRelative(p string) bool {
	return SafeLexical(p) && p != "." && !filepath.IsAbs(p) && filepath.IsLocal(p) && filepath.Clean(p) == p
}
func Within(root, p string) bool {
	r, e := filepath.Rel(root, p)
	return e == nil && (r == "." || filepath.IsLocal(r))
}

// CheckDescendants rejects links/special nodes even if a link would stay inside
// the anchor. Missing tails are permitted only for creation paths.
func CheckDescendants(anchor, rel string, missing bool) error {
	if !SafeRelative(rel) {
		return Err("PATH_UNSAFE", "unsafe relative path")
	}
	r, err := os.OpenRoot(anchor)
	if err != nil {
		return Err("PATH_UNSAFE", "anchor unavailable")
	}
	defer r.Close()
	parts := strings.Split(rel, string(filepath.Separator))
	path := ""
	for n, part := range parts {
		path = filepath.Join(path, part)
		st, e := r.Lstat(path)
		if os.IsNotExist(e) && missing {
			return nil
		}
		if e != nil {
			return e
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() && !st.IsDir() {
			return Err("PATH_UNSAFE", "links and special files are refused")
		}
		if n < len(parts)-1 && !st.IsDir() {
			return Err("PATH_UNSAFE", "non-directory ancestor")
		}
	}
	return nil
}
func Read(root, rel string, limit int64) ([]byte, error) {
	if err := CheckDescendants(root, rel, false); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := r.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, Err("PATH_UNSAFE", "expected regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, Err("OUTPUT_LIMIT", "file exceeds limit")
	}
	return b, nil
}
func ReadAbsolute(path string, limit int64) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	anchor, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	return Read(anchor, filepath.Base(abs), limit)
}

func CheckAbsolute(path string, missing bool) error {
	if !filepath.IsAbs(path) {
		return Err("PATH_UNSAFE", "expected absolute path")
	}
	return CheckDescendants(string(filepath.Separator), strings.TrimPrefix(path, string(filepath.Separator)), missing)
}
