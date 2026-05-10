package projectinit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Prompter asks the user a yes/no question. Returning (true, nil) means yes.
type Prompter interface {
	Confirm(prompt string) (bool, error)
}

// Options configures a single Run invocation.
type Options struct {
	ProjectDir string
	Endpoint   string
	Force      bool
	Print      bool
	Stdout     io.Writer
	Stderr     io.Writer
	Prompter   Prompter
	Prober     Prober
}

const probeTimeout = 500 * time.Millisecond

// Run executes the cco init flow for a single project directory.
func Run(opts Options) error {
	if opts.ProjectDir == "" {
		return errors.New("ProjectDir is required")
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	settingsPath := filepath.Join(opts.ProjectDir, ".claude", "settings.json")
	existing, err := loadExisting(settingsPath)
	if err != nil {
		return fmt.Errorf("load existing settings: %w", err)
	}

	basename := filepath.Base(opts.ProjectDir)

	merged, conflicts, err := MergeSettings(existing, basename, opts.Force)
	if err != nil {
		return err
	}

	if !opts.Force && !opts.Print && len(conflicts) > 0 {
		for _, c := range conflicts {
			fmt.Fprintf(opts.Stdout, "  %s: %q → %q\n", c.Key, c.Existing, c.Proposed)
		}
		ok, err := opts.Prompter.Confirm("Overwrite the keys above? [y/N]")
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		if ok {
			merged, _, err = MergeSettings(existing, basename, true)
			if err != nil {
				return err
			}
		}
	}

	rendered, err := renderIndented(merged)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	if opts.Print {
		_, _ = opts.Stdout.Write(rendered)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("mkdir .claude: %w", err)
	}
	if err := os.WriteFile(settingsPath, rendered, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "✓ wrote %s (%d keys)\n", settingsPath, len(OwnedKeys()))
	fmt.Fprintf(opts.Stdout, "✓ project.name = %s\n", basename)

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout+500*time.Millisecond)
	defer cancel()
	if err := opts.Prober.Probe(ctx, opts.Endpoint, probeTimeout); err != nil {
		fmt.Fprintf(opts.Stdout, "✗ daemon not reachable at %s\n", opts.Endpoint)
		fmt.Fprintln(opts.Stdout, "  → start it with: cco serve")
		fmt.Fprintln(opts.Stdout, "    or install as a service: see README §Install")
	} else {
		fmt.Fprintf(opts.Stdout, "✓ daemon reachable at %s\n", opts.Endpoint)
	}
	return nil
}

func loadExisting(path string) (*OrderedObject, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return NewOrderedObject(), nil
	}
	if err != nil {
		return nil, err
	}
	o := NewOrderedObject()
	if len(bytes.TrimSpace(b)) == 0 {
		return o, nil
	}
	if err := json.Unmarshal(b, o); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return o, nil
}

func renderIndented(o *OrderedObject) ([]byte, error) {
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return nil, err
	}
	pretty.WriteByte('\n')
	return pretty.Bytes(), nil
}
