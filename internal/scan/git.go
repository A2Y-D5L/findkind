package scan

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/a2y-d5l/findkind/internal/config"
	"github.com/a2y-d5l/findkind/internal/manifest"
	"github.com/a2y-d5l/findkind/internal/util"
	"golang.org/x/sync/errgroup"
)

func scanRepo(
	ctx context.Context,
	out io.Writer,
	repo string,
	cfg *config.Config,
	sem *util.Semaphore,
	res *bucket,
) error {
	branches, err := listBranches(ctx, repo)
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	for _, br := range branches {
		if !branchMatches(br, cfg.BranchWords) {
			continue
		}
		branch := br
		if sem.TryAcquire() {
			eg.Go(func() error {
				defer sem.Release()
				return scanBranch(ctx, out, repo, branch, cfg, res)
			})
		} else {
			if err := scanBranch(ctx, out, repo, branch, cfg, res); err != nil {
				return err
			}
		}
	}
	return eg.Wait()
}

// listBranches returns all local + remote branch names excluding symbolic HEAD.
func listBranches(ctx context.Context, repo string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "for-each-ref",
		"--exclude=refs/remotes/*/HEAD", "--format=%(refname:short)",
		"refs/heads", "refs/remotes")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var branches []string
	for scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" && !strings.Contains(v, " -> ") {
			branches = append(branches, v)
		}
	}
	return branches, scanner.Err()
}

func branchMatches(branch string, words []string) bool {
	if len(words) == 0 {
		return true
	}
	lb := strings.ToLower(branch)
	for _, w := range words {
		if strings.Contains(lb, w) {
			return true
		}
	}
	return false
}

// scanBranch uses git grep -z to shortlist YAML files and git show to fetch blobs.
func scanBranch(
	ctx context.Context,
	out io.Writer,
	repo, branch string,
	cfg *config.Config,
	res *bucket,
) error {
	grepArgs := []string{
		"-C", repo,
		"grep", "-I", "-l", "-z", "-i",
		"-e", fmt.Sprintf("kind:[[:space:]]*%s\\b", cfg.Kind),
		branch, "--", "*.yaml", "*.yml",
	}

	outb, err := exec.CommandContext(ctx, "git", grepArgs...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil // no matches
		}
		return err
	}

	for _, p := range bytes.Split(outb, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		filePath := string(p)

		blob, err := exec.CommandContext(ctx, "git", "-C", repo,
			"show", fmt.Sprintf("%s:%s", branch, filePath)).Output()
		if err != nil {
			continue
		}
		if manifest.Match(blob, cfg.Group, cfg.Version, cfg.Kind) {
			hit := fmt.Sprintf("%s:%s:%s", repo, branch, filePath)
			if cfg.Stream && !cfg.OutputJSON {
				put(out, cfg, hit)
			} else {
				res.add(hit)
			}
		}
	}
	return nil
}
