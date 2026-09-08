// Command agentbridge turns mudagent's interactive stdin/stdout protocol into a
// pair of files, so a driver that cannot hold a live pipe across steps can still
// play.
//
// WHY THIS EXISTS. mudagent speaks line-in / JSON-out on stdin and stdout and
// has no file-bridge flag of its own (`mudagent --help` lists only -manifest,
// -target, -user, -password). A driver working through discrete tool calls
// cannot keep a writable stdin open between them, so without this the harness
// is only drivable from a process that stays resident for the whole session.
//
//	commands.txt  driver appends one command per line   -> mudagent stdin
//	events.jsonl  mudagent stdout, one JSON event per line, appended
//
// The command file is read by BYTE OFFSET, never truncated and never rewritten,
// so an append is the only write a driver has to perform and a re-read cannot
// replay a command that was already sent.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	agent := flag.String("agent", "", "path to the mudagent binary")
	target := flag.String("target", "", "host:port of the MUD AI port")
	user := flag.String("user", "", "account username")
	password := flag.String("password", "", "account password")
	bridge := flag.String("bridge", "", "bridge directory for commands.txt / events.jsonl")
	idle := flag.Duration("idle", 0, "exit after this long with no new command (0 = never)")
	flag.Parse()

	if *agent == "" || *target == "" || *bridge == "" {
		fmt.Fprintln(os.Stderr, "agentbridge: -agent, -target and -bridge are required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*bridge, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "agentbridge: mkdir bridge: %v\n", err)
		os.Exit(1)
	}

	cmdPath := filepath.Join(*bridge, "commands.txt")
	evtPath := filepath.Join(*bridge, "events.jsonl")

	// Create the command file up front so a driver can append before we poll.
	if f, err := os.OpenFile(cmdPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		f.Close()
	}

	evt, err := os.OpenFile(evtPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentbridge: open events: %v\n", err)
		os.Exit(1)
	}
	defer evt.Close()

	args := []string{"--target", *target}
	if *user != "" {
		args = append(args, "--user", *user)
	}
	if *password != "" {
		args = append(args, "--password", *password)
	}

	proc := exec.Command(*agent, args...)
	stdin, err := proc.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentbridge: stdin pipe: %v\n", err)
		os.Exit(1)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentbridge: stdout pipe: %v\n", err)
		os.Exit(1)
	}
	proc.Stderr = os.Stderr

	if err := proc.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "agentbridge: start mudagent: %v\n", err)
		os.Exit(1)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
		for sc.Scan() {
			// Append immediately and sync: a driver polling this file between
			// tool calls must never read a half-written line or a stale tail.
			fmt.Fprintln(evt, sc.Text())
			evt.Sync()
		}
	}()

	// Poll the command file by offset. Never truncate it: a driver only ever
	// appends, and offset tracking makes a replay impossible.
	var offset int64
	last := time.Now()
	for {
		select {
		case <-done:
			proc.Wait()
			return
		default:
		}

		f, err := os.Open(cmdPath)
		if err == nil {
			if _, err := f.Seek(offset, io.SeekStart); err == nil {
				sc := bufio.NewScanner(f)
				for sc.Scan() {
					line := sc.Text()
					offset += int64(len(line)) + 1
					line = strings.TrimRight(line, "\r")
					if strings.TrimSpace(line) == "" {
						continue
					}
					if strings.TrimSpace(line) == "__QUIT__" {
						stdin.Close()
						proc.Wait()
						return
					}
					fmt.Fprintln(stdin, line)
					last = time.Now()
				}
			}
			f.Close()
		}

		if *idle > 0 && time.Since(last) > *idle {
			stdin.Close()
			proc.Wait()
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}
