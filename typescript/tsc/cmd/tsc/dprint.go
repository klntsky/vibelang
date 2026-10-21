package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/microsoft/TypeScript/tsc/internal/vibelang"
	"github.com/microsoft/TypeScript/tsc/internal/vibelangfmt"
)

// runDprintPlugin runs the compiler as dprint's process plugin for .vibe
// files. dprint starts a process plugin's executable as
// `<executable> --parent-pid <pid> [--init]`; the plugin exits when that
// process does.
func runDprintPlugin(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "vibelang: --parent-pid needs the process id of dprint")
		return 1
	}
	parent, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "vibelang: --parent-pid %q is not a process id\n", args[1])
		return 1
	}
	if processAliveSupported {
		go func() {
			for range time.Tick(5 * time.Second) {
				if !isProcessAlive(parent) {
					os.Exit(0)
				}
			}
		}()
	}
	err = vibelangfmt.ServePlugin(os.Stdin, os.Stdout, vibelangfmt.PluginOptions{
		Version: vibelang.Version,
		License: "VibeLang is licensed under the Apache License, Version 2.0. Its compiler is built on TypeScript (typescript-go), licensed under the Apache License, Version 2.0.",
		HelpURL: "https://vibelang.sh/setup/formatting",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "vibelang: dprint plugin: %v\n", err)
		return 1
	}
	return 0
}

// runWriteDprintPlugin writes the dprint plugin of the vibelang package at
// args[0] (packages/compiler in a checkout) from this executable.
func runWriteDprintPlugin(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: vibelang --write-dprint-plugin <vibelang package directory>")
		return 1
	}
	executable, err := os.Executable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "vibelang: cannot find this executable: %v\n", err)
		return 1
	}
	manifest, err := vibelangfmt.WritePlugin(args[0], executable, vibelang.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vibelang: cannot write the dprint plugin: %v\n", err)
		return 1
	}
	fmt.Println(manifest)
	return 0
}
