package main

import (
	"strings"
	"time"
)

// enoughCompleted is how many successfully completed builds it takes (without
// --count) to assume shaman has queued every build for the sha1, so that only
// the builds already known need to finish and settle can be skipped.
const enoughCompleted = 3

// Verdict is the outcome of evaluating one snapshot of builds.
type Verdict struct {
	Done    bool
	Failed  bool          // at least one build failed (only meaningful if Done)
	Missing int           // expected builds not yet registered
	Pending int           // builds not yet in a terminal state
	Wait    time.Duration // remaining settle time, if that is all that's left
}

// evaluate decides whether the sha1 is done. lastChange is when the build set
// last changed; settle is how long it must stay unchanged once all builds are
// terminal. If count > 0, settle is skipped once that many builds are present
// and all of them are terminal. Otherwise settle is also skipped once at least
// enoughCompleted builds have completed successfully.
func evaluate(builds []Build, count int, lastChange, now time.Time, settle time.Duration) Verdict {
	var v Verdict
	completed := 0
	for _, b := range builds {
		if !b.Terminal() {
			v.Pending++
		}
		switch b.Status {
		case "failed":
			v.Failed = true
		case "completed":
			completed++
		}
	}
	v.Missing = max(0, count-len(builds))
	if len(builds) == 0 || v.Pending > 0 || v.Missing > 0 {
		return v
	}
	if count > 0 || completed >= enoughCompleted {
		v.Done = true
		return v
	}
	if quiet := now.Sub(lastChange); quiet < settle {
		v.Wait = settle - quiet
		return v
	}
	v.Done = true
	return v
}

// diff returns human-readable status transitions from prev to cur.
func diff(prev, cur []Build) []string {
	old := map[string]string{}
	for _, b := range prev {
		old[b.Key()] = b.Status
	}
	var lines []string
	for _, b := range cur {
		was, ok := old[b.Key()]
		switch {
		case !ok:
			lines = append(lines, b.Key()+": "+b.Status+" (new)")
		case was != b.Status:
			lines = append(lines, b.Key()+": "+was+" → "+b.Status)
		}
	}
	return lines
}

// fingerprint captures everything whose change should reset the settle timer.
func fingerprint(builds []Build) string {
	var sb strings.Builder
	for _, b := range builds {
		sb.WriteString(b.Key() + "=" + b.Status + "@" + b.Modified + ";")
	}
	return sb.String()
}
