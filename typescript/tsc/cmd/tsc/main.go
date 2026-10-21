package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/execute"
	"github.com/microsoft/TypeScript/tsc/internal/osutil"

	// VibeLang registers its compiler hooks from init.
	"github.com/microsoft/TypeScript/tsc/internal/vibelang"
)

func main() {
	os.Exit(runMain())
}

func runMain() int {
	core.ApplyDebugStackLimit()
	args := osutil.Args()[1:]
	if len(args) > 0 {
		switch args[0] {
		case "--lsp":
			return runLSP(args[1:])
		case "--api":
			return runAPI(args[1:])
		case "--parent-pid":
			return runDprintPlugin(args) // dprint starts its process plugins so (dprint.go)
		case "--write-dprint-plugin":
			return runWriteDprintPlugin(args[1:])
		case "--estree":
			return runESTree(args[1:]) // VibeLang ESLint parser (estree.go)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	sys := newSystem()
	if rest, fix := vibelang.FixArguments(args); fix {
		// vibelang --fix applies the safe quick fixes, then compiles (D291).
		if status := vibelang.Fix(ctx, sys, rest); status != 0 {
			return status
		}
		args = rest
	}
	result := execute.CommandLine(ctx, sys, args, nil)
	return int(result.Status)
}
