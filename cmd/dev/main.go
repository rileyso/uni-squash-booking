// Command dev runs the synthetic local web service and restarts it when source
// files change. It is a development tool, not part of the production binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const pollInterval = 500 * time.Millisecond

var watchedExtensions = map[string]bool{
	".css":  true,
	".go":   true,
	".html": true,
	".js":   true,
	".json": true,
	".sql":  true,
	".yaml": true,
	".yml":  true,
}

func main() {
	log.SetFlags(0)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	state, err := snapshot(".")
	if err != nil {
		log.Fatal(err)
	}

	var server *exec.Cmd
	start := func() {
		goBinary := os.Getenv("GO_BINARY")
		if goBinary == "" {
			goBinary = "go"
		}
		server = exec.Command(goBinary, "run", "./cmd/web")
		server.Env = append(os.Environ(), "APP_ENV=development", "GOCACHE=/tmp/syd-squash-go-cache")
		server.Stdout = os.Stdout
		server.Stderr = os.Stderr
		server.Stdin = os.Stdin
		server.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := server.Start(); err != nil {
			log.Fatalf("start development server: %v", err)
		}
		log.Printf("dev: server started (pid %d)", server.Process.Pid)
		go func(command *exec.Cmd) {
			if err := command.Wait(); err != nil && ctx.Err() == nil {
				log.Printf("dev: server stopped: %v", err)
			}
		}(server)
	}
	stopServer := func() {
		if server == nil || server.Process == nil {
			return
		}
		// The Go command and the compiled child share a process group.
		_ = syscall.Kill(-server.Process.Pid, syscall.SIGTERM)
		server = nil
	}

	start()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stopServer()
			return
		case <-ticker.C:
			next, err := snapshot(".")
			if err != nil {
				log.Printf("dev: scan failed: %v", err)
				continue
			}
			if next != state {
				state = next
				log.Print("dev: source change detected; restarting server")
				stopServer()
				start()
			}
		}
	}
}

func snapshot(root string) (string, error) {
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "data" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !watchedExtensions[filepath.Ext(path)] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s:%d:%d", path, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	if len(entries) == 0 {
		return "", errors.New("no source files found to watch")
	}
	return strings.Join(entries, "\n"), nil
}
