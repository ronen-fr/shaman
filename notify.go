package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
)

func printSummary(w io.Writer, builds []Build) {
	sorted := append([]Build(nil), builds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key() < sorted[j].Key() })
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DISTRO\tVERSION\tARCH\tFLAVOR\tSTATUS")
	// All distros of a ceph-dev-pipeline run share one Jenkins job, and shaman
	// gives failed builds that job's whole console log, so group by log URL
	// rather than repeating the same link for every failure.
	var logs []string
	failed := map[string][]string{}
	for _, b := range sorted {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", b.Distro, b.DistroVersion, b.DistroArch, b.Flavor, b.Status)
		if b.Status == "failed" {
			log := cleanURL(b.LogURL)
			if _, ok := failed[log]; !ok {
				logs = append(logs, log)
			}
			failed[log] = append(failed[log], b.Key())
		}
	}
	tw.Flush()
	if len(logs) > 0 {
		fmt.Fprintln(w, "\nfailure logs:")
		for _, log := range logs {
			shown := log
			if shown == "" {
				shown = "(no log URL)"
			}
			fmt.Fprintf(w, "  %s\n    %s\n", shown, strings.Join(failed[log], ", "))
		}
	}
}

var multiSlash = regexp.MustCompile(`/{2,}`)

// cleanURL collapses repeated slashes in the path: shaman builds log URLs as
// url + "/consoleFull" where url already ends in a slash.
func cleanURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Path == "" {
		return s
	}
	u.Path = multiSlash.ReplaceAllString(u.Path, "/")
	u.RawPath = ""
	return u.String()
}

func desktopNotify(ctx context.Context, title, body string) {
	if err := exec.CommandContext(ctx, "notify-send", title, body).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "notify-send:", err)
	}
}

// runHook runs cmd via the shell with the result exported as env vars.
func runHook(ctx context.Context, cmd, branch, sha1, status string) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	c.Env = append(os.Environ(), "BRANCH="+branch, "SHA1="+sha1, "STATUS="+status)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "--exec:", err)
	}
}
