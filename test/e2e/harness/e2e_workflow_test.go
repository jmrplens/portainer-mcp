package harness

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// This file pins the COMMITTED .github/workflows/e2e.yml, the same way
// compose_scripts_test.go and k3d_scripts_test.go pin the committed scripts,
// and for the same reason: nothing else in this repository executes that file,
// so every property it carries is otherwise held up by a comment alone.
//
// The property being pinned is not stylistic. The Business Edition licence
// this project runs on permits one Portainer instance at a time. Two legs
// activating it at once is not a slow test or a flaky one — it is a real
// activation registered against a real, shared account, and the workflow
// did exactly that on every pull request from the day the Kubernetes leg was
// added until the split these tests guard.
//
// test/e2e/.licence.lock cannot guard it. The lock is a file on one runner's
// own filesystem: it refuses a second activation by the same run and is blind
// to every other runner and every other workflow run. Job ordering, and the
// workflow-level concurrency group, are the only mechanisms that serialise
// those — which is precisely why a future reader who sees two
// independent-looking jobs and removes `needs:` to run them in parallel would
// find every test still green, and the licence activated twice.

// workflowFile is the parsed shape of the slice of e2e.yml these tests read:
// the workflow-level concurrency block and, per job, its dependencies and the
// shell of every step.
type workflowFile struct {
	Concurrency struct {
		Group string `yaml:"group"`
		// A pointer so "absent" and "explicitly false" are distinguishable:
		// GitHub's own default is false, but a default is not a decision, and
		// this one has to be a decision.
		CancelInProgress *bool `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]struct {
		Needs yaml.Node `yaml:"needs"` // a scalar or a sequence, per GitHub's schema
		Steps []struct {
			Name string `yaml:"name"`
			If   string `yaml:"if"`
			Run  string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// needsOf returns the job names a job declares as dependencies, accepting
// both spellings GitHub allows: `needs: compose` and `needs: [compose]`.
func needsOf(node yaml.Node) []string {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value == "" {
			return nil
		}
		return []string{node.Value}
	case yaml.SequenceNode:
		out := make([]string, 0, len(node.Content))
		for _, child := range node.Content {
			out = append(out, child.Value)
		}
		return out
	default:
		return nil
	}
}

// loadE2EWorkflow reads and parses the committed workflow. It fails the test
// rather than skipping when the file is missing: a workflow this suite's
// whole licence discipline depends on cannot be optional.
func loadE2EWorkflow(t *testing.T) workflowFile {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".github", "workflows", "e2e.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var wf workflowFile
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(wf.Jobs) == 0 {
		t.Fatalf("%s declares no jobs; every assertion below would pass vacuously", path)
	}
	return wf
}

// repoRoot walks up from this package's directory until it finds the go.mod
// that marks the module root, rather than counting "../.." by hand: the count
// is silently wrong the moment this package moves, and a test that cannot
// find the file it guards is a test that stops guarding it.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above this package: cannot locate the repository root")
		}
		dir = parent
	}
}

// runScript concatenates every step's shell in the named job, so a test can
// ask what a job does without depending on which step does it.
func runScript(t *testing.T, wf workflowFile, job string) string {
	t.Helper()
	j, ok := wf.Jobs[job]
	if !ok {
		t.Fatalf("the e2e workflow has no %q job; it declares %v", job, jobNames(wf))
	}
	var b strings.Builder
	for _, step := range j.Steps {
		b.WriteString(step.Run)
		b.WriteString("\n")
	}
	return b.String()
}

func jobNames(wf workflowFile) []string {
	names := make([]string, 0, len(wf.Jobs))
	for name := range wf.Jobs {
		names = append(names, name)
	}
	return names
}

// TestUnit_E2EWorkflow_KubernetesJobWaitsForTheComposeJob is the guard on the
// ordering. The two jobs must never run concurrently: each activates the same
// single-instance Business Edition licence, and only their ordering keeps that
// to one activation at a time.
func TestUnit_E2EWorkflow_KubernetesJobWaitsForTheComposeJob(t *testing.T) {
	t.Parallel()
	wf := loadE2EWorkflow(t)

	k8s, ok := wf.Jobs["kubernetes"]
	if !ok {
		t.Fatalf("the e2e workflow has no %q job; it declares %v", "kubernetes", jobNames(wf))
	}
	needs := needsOf(k8s.Needs)
	if !slices.Contains(needs, "compose") {
		t.Errorf("the kubernetes job needs %v, want it to include %q: without that ordering both jobs "+
			"activate the same single-instance Business Edition licence at the same time, and "+
			"test/e2e/.licence.lock cannot see across two runners", needs, "compose")
	}
}

// TestUnit_E2EWorkflow_EachJobBringsUpOneLegOnly is the other half of the
// split: sequencing two jobs achieves nothing if one of them still brings up
// both legs, which is exactly the shape this replaced.
func TestUnit_E2EWorkflow_EachJobBringsUpOneLegOnly(t *testing.T) {
	t.Parallel()
	wf := loadE2EWorkflow(t)

	compose := runScript(t, wf, "compose")
	kubernetes := runScript(t, wf, "kubernetes")

	// "make e2e-up" is a prefix of nothing else, but "make e2e-k8s-up" does
	// not contain it, so the two can be told apart by substring safely.
	if !strings.Contains(compose, "make e2e-up") {
		t.Error("the compose job never runs `make e2e-up`: it provisions no estate to test against")
	}
	if strings.Contains(compose, "make e2e-k8s-up") {
		t.Error("the compose job brings up the Kubernetes leg too: that is the single-job shape whose " +
			"two Business Edition activations this split exists to undo")
	}
	if !strings.Contains(kubernetes, "make e2e-k8s-up") {
		t.Error("the kubernetes job never runs `make e2e-k8s-up`: it provisions no Kubernetes leg to test against")
	}
	if strings.Contains(kubernetes, "make e2e-up\n") || strings.Contains(kubernetes, "make e2e-up ") {
		t.Error("the kubernetes job brings the compose estate up as well: with a licence in .env that is " +
			"a second activation, in the one job that was supposed to hold the licence alone")
	}
}

// TestUnit_E2EWorkflow_EveryJobReleasesTheLicenceOnEveryPath pins the
// teardown each job owes: a licence released and the .env carrying the key
// removed, on the failure path as well as the success one.
func TestUnit_E2EWorkflow_EveryJobReleasesTheLicenceOnEveryPath(t *testing.T) {
	t.Parallel()
	wf := loadE2EWorkflow(t)

	for _, tc := range []struct{ job, teardown string }{
		{job: "compose", teardown: "make e2e-down"},
		{job: "kubernetes", teardown: "make e2e-k8s-down"},
	} {
		t.Run(tc.job, func(t *testing.T) {
			t.Parallel()
			j := wf.Jobs[tc.job]
			var found bool
			for _, step := range j.Steps {
				if !strings.Contains(step.Run, tc.teardown) {
					continue
				}
				found = true
				// always(), not the default: a job that failed while its
				// estate held the licence is precisely the job that must
				// still give it back.
				if !strings.Contains(step.If, "always()") {
					t.Errorf("the %s job's teardown step %q runs `if: %q`, want always(): a failed run "+
						"still holds the licence", tc.job, step.Name, step.If)
				}
				if !strings.Contains(step.Run, "rm -f .env") {
					t.Errorf("the %s job's teardown does not remove .env; the licence key would outlive "+
						"the job that wrote it", tc.job)
				}
			}
			if !found {
				t.Errorf("the %s job never runs %q: nothing releases its Business Edition licence", tc.job, tc.teardown)
			}
		})
	}
}

// TestUnit_E2EWorkflow_SerialisesRunsAndNeverCancelsThem guards the level
// above the jobs. Two pull requests open at once are two workflow runs on two
// separate runners; the ordering above says nothing about them, and neither
// does the on-disk lock. Only a workflow-level concurrency group does.
//
// cancel-in-progress must be false, explicitly. A cancelled run is a run
// interrupted with a live Portainer server holding the licence: cancelling it
// races its own teardown, and the key can end up attached to a server about
// to be destroyed, recoverable only through `make e2e-licence-release`.
func TestUnit_E2EWorkflow_SerialisesRunsAndNeverCancelsThem(t *testing.T) {
	t.Parallel()
	wf := loadE2EWorkflow(t)

	if wf.Concurrency.Group == "" {
		t.Error("the e2e workflow declares no concurrency group: two pull requests would run it at the " +
			"same time and activate the same single-instance licence on two estates at once")
	}
	// Keyed by nothing that varies: the resource being serialised is the one
	// licence the whole repository shares, not a branch or a ref.
	if strings.Contains(wf.Concurrency.Group, "${{") {
		t.Errorf("the concurrency group %q is interpolated, so two different refs get two different "+
			"groups and run concurrently; the licence is repository-wide", wf.Concurrency.Group)
	}
	switch {
	case wf.Concurrency.CancelInProgress == nil:
		t.Error("cancel-in-progress is unset: GitHub's default happens to be false, but this has to be a " +
			"decision on the record, because cancelling a run mid-estate can strand the licence")
	case *wf.Concurrency.CancelInProgress:
		t.Error("cancel-in-progress is true: a cancelled run races its own teardown and can leave the " +
			"Business Edition licence attached to a server that is then destroyed")
	}
}
