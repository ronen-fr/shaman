// shaman-watch waits until shaman.ceph.com has finished building the latest
// sha1 of a branch, then reports the result.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	exitOK     = 0
	exitFailed = 1
	exitError  = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		interval = flag.Duration("interval", MinGap, "poll interval (minimum 2m)")
		settle   = flag.Duration("settle", 20*time.Minute, "quiet period after all known builds finish, to catch late-queued builds")
		expect   = flag.String("expect", "", "comma-separated builds that must exist, as distro[/version|codename[/arch[/flavor]]]; skips --settle once all are present and finished")
		sha1Flag = flag.String("sha1", "", "watch this sha1 instead of the branch head")
		repos    = flag.String("repo", "https://github.com/ceph/ceph-ci,https://github.com/ceph/ceph", "comma-separated git repos to resolve the branch head from, first match wins")
		prefix   = flag.String("prefix", "wip-rf-", "when no branch is given, watch the most recently committed branch with this prefix")
		shaman   = flag.String("shaman", "https://shaman.ceph.com", "shaman base URL")
		project  = flag.String("project", "ceph", "shaman project")
		notify   = flag.Bool("notify", false, "send a desktop notification (notify-send) when done")
		hook     = flag.String("exec", "", "shell command to run when done; gets BRANCH, SHA1, STATUS in env")
		timeout  = flag.Duration("timeout", 12*time.Hour, "give up after this long")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] [branch]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		return exitError
	}
	if *interval < MinGap {
		fmt.Fprintf(os.Stderr, "--interval must be at least %s\n", MinGap)
		return exitError
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, cancelTimeout := context.WithTimeout(ctx, *timeout)
	defer cancelTimeout()

	repoList := strings.Split(*repos, ",")
	branch := flag.Arg(0)
	if branch == "" {
		var err error
		if branch, err = newestBranch(ctx, repoList[0], *prefix); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return exitError
		}
		logf("no branch given; using newest %s* branch: %s", *prefix, branch)
	}

	client := NewClient(*shaman, *project)
	expectations := parseExpect(*expect)

	var (
		sha1       string
		prev       []Build
		prevFP     string
		lastChange = time.Now()
	)
	for {
		head := *sha1Flag
		if head == "" {
			h, repo, err := latestSHA1(ctx, repoList, branch)
			if err != nil {
				return fail(ctx, err)
			}
			if h != sha1 {
				logf("watching %s @ %s (from %s)", branch, h, repo)
			}
			head = h
		}
		if head != sha1 {
			if sha1 != "" {
				logf("branch moved: %s → %s; starting over", sha1, head)
			}
			sha1, prev, prevFP, lastChange = head, nil, "", time.Now()
		}

		builds, err := client.Builds(ctx, branch, sha1)
		if err != nil {
			if ctx.Err() != nil {
				return fail(ctx, ctx.Err())
			}
			// Transient errors are worth retrying; just skip this round.
			logf("shaman: %v", err)
		} else {
			for _, line := range diff(prev, builds) {
				logf("  %s", line)
			}
			if fp := fingerprint(builds); fp != prevFP {
				prevFP, lastChange = fp, time.Now()
			}
			prev = builds

			v := evaluate(builds, expectations, lastChange, time.Now(), *settle)
			if v.Done {
				return finish(ctx, branch, sha1, builds, v, *notify, *hook)
			}
			logf("%s", progress(builds, v))
		}

		select {
		case <-ctx.Done():
			return fail(ctx, ctx.Err())
		case <-time.After(*interval):
		}
	}
}

func progress(builds []Build, v Verdict) string {
	switch {
	case len(builds) == 0:
		return "no builds registered yet"
	case v.Pending > 0:
		return fmt.Sprintf("%d/%d builds still running", v.Pending, len(builds))
	case len(v.Missing) > 0:
		return fmt.Sprintf("all %d builds finished; still expecting %v", len(builds), v.Missing)
	default:
		return fmt.Sprintf("all %d builds finished; waiting %s for late builds", len(builds), v.Wait.Round(time.Second))
	}
}

func finish(ctx context.Context, branch, sha1 string, builds []Build, v Verdict, notify bool, hook string) int {
	status, code := "success", exitOK
	if v.Failed {
		status, code = "failed", exitFailed
	}
	fmt.Println()
	fmt.Printf("%s @ %s: %s\n", branch, sha1, status)
	printSummary(os.Stdout, builds)

	// Use a fresh context so the hooks still run if we were near the timeout.
	hctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	defer cancel()
	if notify {
		desktopNotify(hctx, "shaman: "+branch+" "+status, fmt.Sprintf("%s: %d builds", sha1[:min(8, len(sha1))], len(builds)))
	}
	if hook != "" {
		runHook(hctx, hook, branch, sha1, status)
	}
	return code
}

func fail(ctx context.Context, err error) int {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = errors.New("timed out")
	}
	fmt.Fprintln(os.Stderr, "shaman-watch:", err)
	return exitError
}

func logf(format string, args ...any) {
	fmt.Printf("%s "+format+"\n", append([]any{time.Now().Format("15:04:05")}, args...)...)
}
