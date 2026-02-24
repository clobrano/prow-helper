package prowapi

import (
	"net/url"
	"testing"
)

const sampleProwJobsJS = `var allBuilds = {
  "items": [
    {
      "spec": {
        "job": "pull-ci-openshift-cno-master-e2e-aws-ovn",
        "type": "presubmit",
        "refs": {
          "pulls": [{"author": "clobrano", "number": 42, "sha": "abc"}]
        }
      },
      "status": {
        "state": "pending",
        "url": "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/pull-ci-openshift-cno-master-e2e-aws-ovn/1234567890",
        "build_id": "1234567890"
      }
    },
    {
      "spec": {
        "job": "pull-ci-openshift-cno-master-unit",
        "type": "presubmit",
        "refs": {
          "pulls": [{"author": "other-user", "number": 43, "sha": "def"}]
        }
      },
      "status": {
        "state": "success",
        "url": "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/pull-ci-openshift-cno-master-unit/9876543210",
        "build_id": "9876543210"
      }
    },
    {
      "spec": {"job": "periodic-nightly", "type": "periodic"},
      "status": {
        "state": "triggered",
        "url": "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-nightly/1111111111",
        "build_id": "1111111111"
      }
    },
    {
      "spec": {"job": "no-url-job", "type": "periodic"},
      "status": {"state": "pending", "url": "", "build_id": "0000000000"}
    }
  ]
}`

func TestParse(t *testing.T) {
	jobs, err := parse([]byte(sampleProwJobsJS))
	if err != nil {
		t.Fatalf("parse() returned unexpected error: %v", err)
	}
	// "no-url-job" has an empty URL and must be skipped.
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs (empty-URL job skipped), got %d", len(jobs))
	}

	j := jobs[0]
	if j.Name != "pull-ci-openshift-cno-master-e2e-aws-ovn" {
		t.Errorf("unexpected job name: %s", j.Name)
	}
	if j.BuildID != "1234567890" {
		t.Errorf("unexpected build ID: %s", j.BuildID)
	}
	if j.Author != "clobrano" {
		t.Errorf("unexpected author: %s", j.Author)
	}
	if j.State != "pending" {
		t.Errorf("unexpected state: %s", j.State)
	}

	// Periodic job has no pulls so Author must be empty.
	if jobs[2].Author != "" {
		t.Errorf("periodic job should have empty author, got: %s", jobs[2].Author)
	}
}

func TestParseStripsJSPrefix(t *testing.T) {
	variants := []string{
		`var allBuilds = {"items":[]}`,
		`var foo = {"items":[]}`,
		`{"items":[]}`,
		`var allBuilds = {"items":[]};`,
	}
	for _, v := range variants {
		jobs, err := parse([]byte(v))
		if err != nil {
			t.Errorf("parse(%q) error: %v", v, err)
		}
		if len(jobs) != 0 {
			t.Errorf("parse(%q) expected 0 jobs, got %d", v, len(jobs))
		}
	}
}

func TestFilter(t *testing.T) {
	jobs, _ := parse([]byte(sampleProwJobsJS))

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{"author filter", "author=clobrano", 1},
		{"state filter", "state=success", 1},
		{"job substring filter", "job=unit", 1},
		{"no filter", "", 3},
		{"author not found", "author=nobody", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, _ := url.ParseQuery(tt.query)
			result := filter(jobs, q)
			if len(result) != tt.wantCount {
				t.Errorf("filter(%q): got %d jobs, want %d", tt.query, len(result), tt.wantCount)
			}
		})
	}
}
