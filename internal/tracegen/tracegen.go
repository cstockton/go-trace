// Package tracegen provides internal utilities.
package tracegen

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Generator is internal and should not procuce a lint warning.
type Generator struct {
	P, Bin string
	N, S   int
}

// Run is internal and should not procuce a lint warning.
func (g Generator) Run(ctx context.Context, w io.Writer) error {
	if err := g.Build(ctx); err != nil {
		return err
	}

	count, size := fmt.Sprintf(`%d`, g.N), fmt.Sprintf(`%d`, g.S)
	cmd := exec.CommandContext(ctx, g.Bin, "-n", count, "-s", size)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	if _, err = io.Copy(w, stdout); err != nil {
		return err
	}
	return cmd.Wait()
}

// Build is internal and should not procuce a lint warning.
func (g *Generator) Build(ctx context.Context) error {
	stat, err := os.Stat(g.Bin)
	if err == nil && stat.IsDir() {
		return fmt.Errorf(`Bin was dir: %v`, g.Bin)
	}
	if err == nil {
		return nil
	}

	cur, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		if err = os.Chdir(cur); err != nil {
			panic(fmt.Errorf(`unable to restore work dir: %v`, err))
		}
	}()
	if err = os.Chdir(g.P); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "go", "build", "tracegen.go")
	return cmd.Run()
}

// New is internal and should not procuce a lint warning.
func New(p string) (*Generator, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	g := &Generator{P: abs, Bin: filepath.Join(abs, `tracegen`), N: 10, S: 256}
	return g, nil
}
