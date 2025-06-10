package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/a2y-d5l/findkind/internal/config"
	"github.com/a2y-d5l/findkind/internal/manifest"
	"github.com/a2y-d5l/findkind/internal/util"
	"golang.org/x/sync/errgroup"
)

// put writes one hit to out in the format requested by cfg.
func put(out io.Writer, cfg *config.Config, record string) {
	switch {
	case cfg.JSONLines:
		_ = json.NewEncoder(out).Encode(struct {
			Path string `json:"path"`
		}{record})
	case cfg.NullTerm:
		_, _ = out.Write([]byte(record))
		_, _ = out.Write([]byte{0})
	default:
		fmt.Fprintln(out, record)
	}
}

// Result collection when --stream=false OR --output-json=true
type bucket struct {
	sync.Mutex
	order []string
	set   map[string]struct{}
}

func newBucket() *bucket {
	return &bucket{set: make(map[string]struct{})}
}

func (b *bucket) add(hit string) {
	b.Lock()
	if _, dup := b.set[hit]; !dup {
		b.set[hit] = struct{}{}
		b.order = append(b.order, hit)
	}
	b.Unlock()
}

// Run performs the scan and writes matches to out.
func Run(ctx context.Context, out io.Writer, cfg *config.Config) error {
	sem := util.NewSemaphore(cfg.MaxProcs)

	var res *bucket
	if !cfg.Stream || cfg.OutputJSON {
		res = newBucket()
	}

	eg, ctx := errgroup.WithContext(ctx)

	walkFn := func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Git repository?
		if d.IsDir() && d.Name() == ".git" && !cfg.NoGit {
			repo := filepath.Dir(path)
			if sem.TryAcquire() {
				eg.Go(func() error {
					defer sem.Release()
					return scanRepo(ctx, out, repo, cfg, sem, res)
				})
			} else {
				if err := scanRepo(ctx, out, repo, cfg, sem, res); err != nil {
					return err
				}
			}
			return filepath.SkipDir
		}

		// Plain YAML file
		if !d.IsDir() && isYAML(path) {
			if sem.TryAcquire() {
				pcopy := path
				eg.Go(func() error {
					defer sem.Release()
					return inspectFile(out, pcopy, cfg, res)
				})
			} else {
				if err := inspectFile(out, path, cfg, res); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := filepath.WalkDir(cfg.Root, walkFn); err != nil {
		return err
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Deferred output modes
	if res != nil {
		if cfg.OutputJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(res.order)
		}
		for _, line := range res.order {
			put(out, cfg, line)
		}
	}

	return nil
}

// --- helpers -----------------------------------------------------------------

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func inspectFile(out io.Writer, path string, cfg *config.Config, res *bucket) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if manifest.Match(data, cfg.Group, cfg.Version, cfg.Kind) {
		if cfg.Stream && !cfg.OutputJSON {
			put(out, cfg, path)
			return nil
		}
		res.add(path)
	}
	return nil
}

// CPUsAvailable returns runtime.GOMAXPROCS(0) in Go 1.22.
func CPUsAvailable() int { return runtime.GOMAXPROCS(0) }
