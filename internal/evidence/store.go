// Package evidence owns append-only private verification records.
package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/akkaraponph/itervyn/internal/config"
	"github.com/akkaraponph/itervyn/internal/verify"
)

const MaxArtifact = 5 << 20

type Store struct{}

func (Store) Begin(ctx context.Context, c config.Resolved, id string) (verify.RunHandle, error) {
	h := verify.RunHandle{RunID: id, Directory: filepath.Join(c.EvidenceDir, id)}
	if ctx.Err() != nil {
		return h, ctx.Err()
	}
	if !config.IDPattern.MatchString(id) {
		return h, config.Err("PATH_UNSAFE", "run identifier")
	}
	if err := config.CheckAbsolute(c.EvidenceDir, true); err != nil {
		return h, err
	}
	r, err := os.OpenRoot(string(filepath.Separator))
	if err != nil {
		return h, err
	}
	defer r.Close()
	rel, err := filepath.Rel(string(filepath.Separator), c.EvidenceDir)
	if err != nil {
		return h, err
	}
	if err = r.MkdirAll(rel, 0700); err != nil {
		return h, err
	}
	if err = r.Mkdir(filepath.Join(rel, id), 0700); err != nil {
		return h, err
	}
	return h, nil
}
func write(h verify.RunHandle, path string, b []byte) error {
	if !config.IDPattern.MatchString(h.RunID) || filepath.Base(h.Directory) != h.RunID || !config.SafeRelative(path) || len(b) > MaxArtifact {
		return config.Err("PATH_UNSAFE", "artifact path or size")
	}
	if err := config.CheckDescendants(h.Directory, path, true); err != nil {
		return err
	}
	r, err := os.OpenRoot(h.Directory)
	if err != nil {
		return err
	}
	defer r.Close()
	parent := filepath.Dir(path)
	if parent != "." {
		if err = r.MkdirAll(parent, 0700); err != nil {
			return err
		}
	}
	f, err := r.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
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
		return err
	}
	d, err := r.Open(parent)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func (Store) Put(ctx context.Context, h verify.RunHandle, a verify.NativeArtifact) (verify.ArtifactRef, error) {
	ref := verify.ArtifactRef{Path: a.Path, SHA256: config.Hash(a.Data), Bytes: int64(len(a.Data))}
	if ctx.Err() != nil {
		return ref, ctx.Err()
	}
	if a.Path == "result.json" || a.Path == "COMMITTED" || a.Path == ".COMMITTED" {
		return ref, config.Err("PATH_UNSAFE", "reserved artifact")
	}
	if _, err := os.Lstat(filepath.Join(h.Directory, "COMMITTED")); err == nil {
		return ref, config.Err("EVIDENCE_MISMATCH", "run already committed")
	}
	return ref, write(h, a.Path, a.Data)
}
func (s Store) Commit(ctx context.Context, h verify.RunHandle, result verify.Result) (err error) {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if result.RunID != h.RunID || !result.EvidencePersisted {
		return config.Err("EVIDENCE_MISMATCH", "result identity")
	}
	if err = verify.ValidateResult(result); err != nil {
		return err
	}
	if err = verifyArtifacts(h.Directory, result); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err = write(h, "result.json", b); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err = write(h, ".COMMITTED", []byte(config.Hash(b)+"\n")); err != nil {
		return err
	}
	root, err := os.OpenRoot(h.Directory)
	if err != nil {
		return err
	}
	defer root.Close()
	defer root.Remove(".COMMITTED")
	published := false
	defer func() {
		if err != nil && published {
			_ = root.Remove("COMMITTED")
		}
	}()
	// This is the publication boundary. Cancellation before it cannot commit PASS.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err = root.Link(".COMMITTED", "COMMITTED"); err != nil {
		return err
	}
	published = true
	for _, p := range []string{h.Directory, filepath.Dir(h.Directory)} {
		d, e := os.Open(p)
		if e != nil {
			return e
		}
		e = d.Sync()
		_ = d.Close()
		if e != nil {
			return e
		}
	}
	return nil
}
func verifyArtifacts(dir string, r verify.Result) error {
	seen := map[string]verify.ArtifactRef{}
	for _, a := range r.Artifacts {
		if !config.SafeRelative(a.Path) || a.Path == "result.json" || a.Path == "COMMITTED" || a.Path == ".COMMITTED" || !config.DigestPattern.MatchString(a.SHA256) || a.Bytes < 0 || a.Bytes > MaxArtifact {
			return config.Err("EVIDENCE_CORRUPT", "artifact reference")
		}
		if _, ok := seen[a.Path]; ok {
			return config.Err("EVIDENCE_CORRUPT", "duplicate artifact")
		}
		seen[a.Path] = a
		b, e := config.Read(dir, a.Path, MaxArtifact)
		if e != nil || int64(len(b)) != a.Bytes || config.Hash(b) != a.SHA256 {
			return config.Err("EVIDENCE_CORRUPT", "artifact bytes")
		}
	}
	for _, p := range r.Providers {
		for _, a := range p.Evidence {
			want, ok := seen[a.Path]
			if !ok || want != a {
				return config.Err("EVIDENCE_CORRUPT", "provider artifact linkage")
			}
		}
	}
	return nil
}
func (Store) Inspect(ctx context.Context, dir, id string) (verify.Result, error) {
	var r verify.Result
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	if !config.IDPattern.MatchString(id) {
		return r, config.Err("PATH_UNSAFE", "run identifier")
	}
	if _, e := os.Stat(dir); os.IsNotExist(e) {
		return r, config.Err("RUN_NOT_FOUND", "run unavailable")
	}
	if err := config.CheckDescendants(dir, id, false); err != nil {
		if os.IsNotExist(err) {
			return r, config.Err("RUN_NOT_FOUND", "run unavailable")
		}
		return r, err
	}
	root := filepath.Join(dir, id)
	marker, err := config.Read(root, "COMMITTED", 100)
	if err != nil {
		if os.IsNotExist(err) {
			return r, config.Err("INCOMPLETE_RUN", "run has no commit marker")
		}
		return r, config.Err("EVIDENCE_CORRUPT", "commit marker")
	}
	b, err := config.Read(root, "result.json", MaxArtifact)
	if err != nil || string(marker) != config.Hash(b)+"\n" {
		return r, config.Err("EVIDENCE_CORRUPT", "result checksum")
	}
	if err = config.DecodeStrict(b, &r); err != nil {
		return r, config.Err("EVIDENCE_CORRUPT", "result schema")
	}
	if r.RunID != id {
		return r, config.Err("EVIDENCE_CORRUPT", "result run identity")
	}
	if err = verify.ValidateResult(r); err != nil {
		return r, err
	}
	if err = verifyArtifacts(root, r); err != nil {
		return r, err
	}
	return r, nil
}
