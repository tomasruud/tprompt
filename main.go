package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func main() {
	args := os.Args[1:]

	var cmd string
	if len(args) > 0 {
		cmd = args[0]
	}

	var err error
	switch cmd {
	case "remote":
		err = printRemote(os.Stdout)

	case "path":
		fs := flag.NewFlagSet("path", flag.ExitOnError)
		width := fs.Int("width", 40, "Max width")
		fs.Parse(args[0:])
		err = printPath(*width, os.Stdin, os.Stdout)

	case "", "prompt":
		err = printPrompt(os.Stdout)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Prompt err (%s): %v\n", cmd, err)
		os.Exit(1)
	}
}

func printRemote(out io.Writer) error {
	if !isRemote() {
		return nil
	}

	usr, err := user.Current()
	if err != nil {
		return err
	}
	out.Write([]byte(usr.Username))

	host, err := os.Hostname()
	if err != nil {
		return err
	}
	out.Write([]byte("@" + host))

	return nil
}

func isRemote() bool {
	return os.Getenv("SSH_CONNECTION") != ""
}

func printPath(width int, in io.Reader, out io.Writer) error {
	fullPath, _ := io.ReadAll(in)
	home, _ := os.UserHomeDir()

	path := formatPath(string(fullPath), width, home)
	out.Write([]byte(path))

	return nil
}

func formatPath(path string, width int, home string) string {
	if strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}

	if len(path) <= width {
		return path
	}

	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) < 1 {
		return path
	}

	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != "~" && len(parts[i]) > 0 {
			parts[i] = string(parts[i][0])
		}

		path = filepath.Join(parts...)
		if len(path) <= width {
			return path
		}
	}

	slices.Reverse(parts)

	path = parts[0]
	for _, p := range parts[1:] {
		v := filepath.Join(p, path)
		if len(v) < width {
			path = v
		} else {
			break
		}
	}

	return "+" + path
}

func printPrompt(out io.Writer) error {
	if isRepo() {
		out.Write([]byte("@"))

		status, err := repoStatus()
		if err != nil {
			return err
		}

		if status.head != "main" && status.head != "master" {
			out.Write([]byte("*"))
		}

		if status.dirty {
			out.Write([]byte("!"))
		}

		if status.behind > 0 {
			out.Write([]byte("<"))
		}

		if status.ahead > 0 {
			out.Write([]byte(">"))
		}

		if status.stash {
			out.Write([]byte("$"))
		}
	}

	out.Write([]byte("> "))

	return nil
}

func isRepo() bool {
	cmd := exec.Command("git", "--no-optional-locks", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

func repoStatus() (status, error) {
	cmd := exec.Command("git", "--no-optional-locks", "status", "--show-stash", "--branch", "--porcelain=v2")
	out, err := cmd.Output()
	if err != nil {
		return status{}, err
	}

	return parseRepoStatus(out)
}

func parseRepoStatus(in []byte) (status, error) {
	var st status
	for _, ln := range strings.Split(string(in), "\n") {
		info := strings.Split(ln, " ")

		if len(info) < 2 {
			continue
		}

		switch info[0] {
		case "#":
			switch info[1] {
			case "branch.head":
				st.head = info[2]

			case "branch.ab":
				st.ahead, _ = strconv.Atoi(info[2][1:])
				st.behind, _ = strconv.Atoi(info[3][1:])

			case "stash":
				n, err := strconv.Atoi(info[2])
				if err != nil {
					continue
				}
				st.stash = n > 0
			}

		case "!", "?", "1", "2", "u":
			st.dirty = true
		}
	}

	return st, nil
}

type status struct {
	head   string
	ahead  int
	behind int
	dirty  bool
	stash  bool
}
