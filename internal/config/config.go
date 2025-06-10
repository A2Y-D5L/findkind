package config

import (
	"errors"
	"flag"
	"io"
	"path/filepath"
	"runtime"
	"strings"
)

// Version is injected at build-time with
// -ldflags="-X github.com/a2y-d5l/findkind/internal/config.Version=$(git describe --tags --always --dirty)"
var Version = "dev"

// Config captures all user-tunable knobs.
type Config struct {
	Root        string
	Group       string
	Version     string
	Kind        string
	BranchWords []string
	MaxProcs    int
	NoGit       bool
	// Output modes
	Stream    bool
	NullTerm  bool
	JSONLines bool
	Quiet     bool
	// legacy
	OutputJSON bool
	Verbose    bool
}

// ParseFlags populates Config from CLI flags.
// Pass stdout so that --help is POSIX-friendly.
func ParseFlags(stdout io.Writer) (*Config, error) {
	flag.CommandLine.SetOutput(stdout)

	var (
		root     = flag.String("path", ".", "root directory to search")
		branches = flag.String("branch-filters", "", "comma-separated keywords that must appear in a branch name")
		group    = flag.String("group", "*", "Kubernetes API group to match")
		apiVer   = flag.String("version", "*", "Kubernetes API version to match")
		kind     = flag.String("kind", "", "Kubernetes Kind to match (required)")

		maxProcs = flag.Int("max-procs", runtime.NumCPU()*4, "maximum concurrent git/YAML workers")
		noGit    = flag.Bool("no-git", false, "disable git branch scanning (disk files only)")

		stream   = flag.Bool("stream", true, "emit results as they are found (recommended)")
		nullTerm = flag.Bool("0", false, "NUL-terminate each record (for xargs -0)")
		jsonl    = flag.Bool("jsonl", false, "output newline-delimited JSON records")
		quiet    = flag.Bool("quiet", false, "suppress non-data output on stdout")

		jsonOut = flag.Bool("output-json", false, "emit one final JSON array of paths (non-streaming)")
		verbose = flag.Bool("verbose", false, "enable verbose logging")

		showVer = flag.Bool("version", false, "print kindfinder version and exit")
	)

	flag.Parse()

	if *showVer {
		_, _ = stdout.Write([]byte("kindfinder " + Version + "\n"))
		return nil, flag.ErrHelp
	}

	// Mandatory –kind
	if strings.TrimSpace(*kind) == "" {
		return nil, errors.New("-kind is required")
	}

	// Validate mutually-exclusive modes
	if (*nullTerm && *jsonl) || (*jsonOut && (*nullTerm || *jsonl)) {
		return nil, errors.New("flags -0, -jsonl and -output-json are mutually exclusive")
	}

	words := make([]string, 0)
	for _, w := range strings.Split(*branches, ",") {
		if w = strings.TrimSpace(w); w != "" {
			words = append(words, strings.ToLower(w))
		}
	}

	return &Config{
		Root:        filepath.Clean(*root),
		Group:       *group,
		Version:     *apiVer,
		Kind:        *kind,
		BranchWords: words,
		MaxProcs:    *maxProcs,
		NoGit:       *noGit,
		Stream:      *stream,
		NullTerm:    *nullTerm,
		JSONLines:   *jsonl,
		Quiet:       *quiet,
		OutputJSON:  *jsonOut,
		Verbose:     *verbose,
	}, nil
}
