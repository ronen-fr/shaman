package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// latestSHA1 returns the head of branch in the first repo that has it.
func latestSHA1(ctx context.Context, repos []string, branch string) (sha1, repo string, err error) {
	for _, r := range repos {
		out, err := exec.CommandContext(ctx, "git", "ls-remote", r, "refs/heads/"+branch).Output()
		if err != nil {
			return "", "", fmt.Errorf("git ls-remote %s: %w", r, err)
		}
		if fields := strings.Fields(string(out)); len(fields) >= 1 {
			return fields[0], r, nil
		}
	}
	return "", "", fmt.Errorf("branch %q not found in %s", branch, strings.Join(repos, ", "))
}

// newestBranch returns the branch in repo starting with prefix whose head
// commit is most recent. It fetches only the head commits (no trees or blobs)
// into a throwaway bare repo, so it is cheap even for large repositories.
func newestBranch(ctx context.Context, repo, prefix string) (string, error) {
	dir, err := os.MkdirTemp("", "shaman-watch-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	refspec := fmt.Sprintf("+refs/heads/%s*:refs/heads/%s*", prefix, prefix)
	steps := [][]string{
		{"init", "-q", "--bare", dir},
		{"-C", dir, "fetch", "-q", "--depth=1", "--filter=tree:0", repo, refspec},
	}
	for _, args := range steps {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
		}
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "for-each-ref",
		"--sort=-committerdate", "--count=1", "--format=%(refname:lstrip=2)", "refs/heads/").Output()
	if err != nil {
		return "", fmt.Errorf("git for-each-ref: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("no branch matching %s* in %s", prefix, repo)
	}
	return branch, nil
}
