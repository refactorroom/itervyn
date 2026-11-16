package verify

import (
	"context"
	"github.com/akkaraponph/itervyn/internal/config"
	"time"
)

type Request struct {
	Config    config.Resolved
	RunID     string
	StartedAt time.Time
}
type NativeArtifact struct {
	Path string
	Data []byte
}
type Observation struct {
	Result    ProviderResult
	Artifacts []NativeArtifact
}
type Provider interface {
	Name() string
	Verify(context.Context, Request) Observation
}
type RunHandle struct {
	RunID     string
	Directory string
}
type Store interface {
	Begin(context.Context, config.Resolved, string) (RunHandle, error)
	Put(context.Context, RunHandle, NativeArtifact) (ArtifactRef, error)
	Commit(context.Context, RunHandle, Result) error
	Inspect(context.Context, string, string) (Result, error)
}
type Service struct {
	Providers []Provider
	Store     Store
}
