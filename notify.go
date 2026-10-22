package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"text/tabwriter"
)

func printSummary(w io.Writer, builds []Build) {
	sorted := append([]Build(nil), builds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key() < sorted[j].Key() })
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "DISTRO\tVERSION\tARCH\tFLAVOR\tSTATUS\tLOG")
	for _, b := range sorted {
		log := ""
		if b.Status == "failed" {
			log = b.LogURL
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", b.Distro, b.DistroVersion, b.DistroArch, b.Flavor, b.Status, log)
	}
	tw.Flush()
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
