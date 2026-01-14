/*
 * Copyright (c) 2022, HabanaLabs Ltd.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/HabanaAI/habana-container-runtime/internal/config"
)

var (
	debugflag  = flag.Bool("debug", false, "enable debug output")
	configflag = flag.String("config", "", "configuration file")

	defaultPATH = []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"}
)

func exit() {
	if err := recover(); err != nil {
		if _, ok := err.(runtime.Error); ok {
			_, err := fmt.Fprintln(os.Stderr, err)
			if err != nil {
				log.Printf("failed to write error to stderr: %v", err)
			}
		}
		if *debugflag {
			_, err := fmt.Fprintf(os.Stderr, "%v\n", debug.Stack())
			if err != nil {
				log.Printf("failed to write error to stderr: %v", err)
			}
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func getPATH(cfg config.CLIConfig) string {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	// directories from the hook environment have higher precedence
	dirs = append(dirs, defaultPATH...)

	if cfg.Root != nil {
		rootDirs := []string{}
		for _, dir := range dirs {
			rootDirs = append(rootDirs, path.Join(*cfg.Root, dir))
		}
		// directories with the root prefix have higher precedence
		dirs = append(rootDirs, dirs...)
	}
	return strings.Join(dirs, ":")
}

func getCLIPath(cfg config.CLIConfig) (string, error) {
	if cfg.Path != nil {
		return *cfg.Path, nil
	}

	if err := os.Setenv("PATH", getPATH(cfg)); err != nil {
		return "", fmt.Errorf("couldn't set PATH variable: %w", err)
	}

	path, err := exec.LookPath("habana-container-cli")
	if err != nil {
		return "", fmt.Errorf("couldn't find binary habana-container-cli in $PATH (%s): %w", os.Getenv("PATH"), err)
	}
	return path, nil
}

// getRootfsPath returns an absolute path. We don't need to resolve symlinks for now.
func getRootfsPath(config containerConfig) string {
	rootfs, err := filepath.Abs(config.Rootfs)
	if err != nil {
		log.Panicln(err)
	}
	return rootfs
}

func doHook(lifecycle string) {
	var err error

	defer exit()
	log.SetFlags(0)

	hook := getHookConfig()
	cli := hook.CLI
	rt := hook.Runtime

	container := getContainerConfig(hook)
	habana := container.Habana
	if habana == nil {
		// Not a HL devices, nothing to do.
		return
	}

	rootfs := getRootfsPath(container)

	args := []string{}
	if len(habana.Devices) > 0 {
		args = append(args, fmt.Sprintf("--device=%s", habana.Devices))
	}
	if cli.Root != nil {
		args = append(args, fmt.Sprintf("--root=%s", *cli.Root))
	}
	if cli.LogFile != "" {
		args = append(args, fmt.Sprintf("--log-file=%s", cli.LogFile))
	}
	if hook.MountAccelerators {
		args = append(args, fmt.Sprintf("--mount-accelerators=%t", hook.MountAccelerators))
	}
	if hook.MountUverbs {
		args = append(args, fmt.Sprintf("--mount-uverbs=%t", hook.MountUverbs))
	}
	if rt.Mode == ModeCDI {
		args = append(args, "--cdi=true")
	}

	args = append(args, fmt.Sprintf("--hook=%s", lifecycle))
	args = append(args, fmt.Sprintf("--pid=%s", strconv.FormatUint(uint64(container.Pid), 10)))
	args = append(args, rootfs)
	env := append(os.Environ(), cli.Environment...)

	cliPath, err := getCLIPath(cli)
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			log.Printf("failed to write error to stderr: %v", err)
		}
		os.Exit(1)
	}

	cmd := exec.Command(cliPath, args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			log.Printf("failed to write error to stderr: %v", err)
		}
		os.Exit(1)
	}
	fmt.Println(string(output))
}

func usage() {
	usage := []string{
		// Format the usage message "Usage of %s:\n"
		"Usage: " + os.Args[0] + " <command>",
		"",
		"Available commands:",
		"  prestart       Run the prestart hook",
		"  createRuntime  Run the createRuntime hook",
		"  poststart      No-op",
		"  poststop       No-op",
	}

	_, err := fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	if err != nil {
		os.Exit(1)
	}
	flag.PrintDefaults()
	for _, line := range usage {
		_, err = fmt.Fprintf(os.Stderr, "  %s\n", line)
		if err != nil {
			os.Exit(1)
		}
	}
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	switch args[0] {
	case "prestart", "createRuntime":
		doHook(args[0])
		os.Exit(0)
	case "poststart":
		fallthrough
	case "poststop":
		os.Exit(0)
	default:
		flag.Usage()
		os.Exit(2)
	}
}
