package parser

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid PROW URL",
			url:     "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296",
			wantErr: false,
		},
		{
			name:    "valid PROW URL with different job",
			url:     "https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/pull-ci-openshift-origin-master-e2e-aws/12345",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL - wrong host",
			url:     "https://example.com/view/gs/test-bucket/logs/job/123",
			wantErr: true,
		},
		{
			name:    "invalid URL - missing gs prefix",
			url:     "https://prow.ci.openshift.org/view/test-bucket/logs/job/123",
			wantErr: true,
		},
		{
			name:    "invalid URL - not a URL",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "invalid URL - missing path components",
			url:     "https://prow.ci.openshift.org/view/gs/",
			wantErr: true,
		},
		{
			name:    "invalid URL - HTTP instead of HTTPS",
			url:     "http://prow.ci.openshift.org/view/gs/test-bucket/logs/job/123",
			wantErr: true,
		},
		{
			name:    "valid URL with trailing slash",
			url:     "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/123456/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantBucket  string
		wantPath    string
		wantJobName string
		wantBuildID string
		wantPRRef   string
		wantErr     bool
	}{
		{
			name:        "valid PROW URL - full path",
			url:         "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296",
			wantBucket:  "test-platform-results",
			wantPath:    "logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296",
			wantJobName: "periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview",
			wantBuildID: "2013057817195319296",
			wantPRRef:   "",
			wantErr:     false,
		},
		{
			name:        "valid PROW URL with trailing slash",
			url:         "https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/pull-ci-openshift-origin-master-e2e-aws/12345/",
			wantBucket:  "origin-ci-test",
			wantPath:    "logs/pull-ci-openshift-origin-master-e2e-aws/12345",
			wantJobName: "pull-ci-openshift-origin-master-e2e-aws",
			wantBuildID: "12345",
			wantPRRef:   "",
			wantErr:     false,
		},
		{
			name:        "valid PROW URL - pr-logs path",
			url:         "https://prow.ci.openshift.org/view/gs/origin-ci-test/pr-logs/pull/openshift_origin/12345/pull-ci-openshift-origin-master-e2e-aws/67890",
			wantBucket:  "origin-ci-test",
			wantPath:    "pr-logs/pull/openshift_origin/12345/pull-ci-openshift-origin-master-e2e-aws/67890",
			wantJobName: "pull-ci-openshift-origin-master-e2e-aws",
			wantBuildID: "67890",
			wantPRRef:   "[openshift/origin PR12345]",
			wantErr:     false,
		},
		{
			name:    "invalid URL",
			url:     "not-a-valid-url",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := ParseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if metadata.Bucket != tt.wantBucket {
				t.Errorf("ParseURL() Bucket = %v, want %v", metadata.Bucket, tt.wantBucket)
			}
			if metadata.Path != tt.wantPath {
				t.Errorf("ParseURL() Path = %v, want %v", metadata.Path, tt.wantPath)
			}
			if metadata.JobName != tt.wantJobName {
				t.Errorf("ParseURL() JobName = %v, want %v", metadata.JobName, tt.wantJobName)
			}
			if metadata.BuildID != tt.wantBuildID {
				t.Errorf("ParseURL() BuildID = %v, want %v", metadata.BuildID, tt.wantBuildID)
			}
			if metadata.PRRef != tt.wantPRRef {
				t.Errorf("ParseURL() PRRef = %v, want %v", metadata.PRRef, tt.wantPRRef)
			}
		})
	}
}

func TestBuildGsutilCommand(t *testing.T) {
	tests := []struct {
		name     string
		metadata *ProwMetadata
		dest     string
		want     string
	}{
		{
			name: "standard path",
			metadata: &ProwMetadata{
				Bucket: "test-platform-results",
				Path:   "logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296",
			},
			dest: "/tmp/artifacts",
			want: "gsutil -m cp -r gs://test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296/ /tmp/artifacts",
		},
		{
			name: "path with home directory",
			metadata: &ProwMetadata{
				Bucket: "origin-ci-test",
				Path:   "logs/pull-ci-openshift-origin-master-e2e-aws/12345",
			},
			dest: "~/prow-artifacts",
			want: "gsutil -m cp -r gs://origin-ci-test/logs/pull-ci-openshift-origin-master-e2e-aws/12345/ ~/prow-artifacts",
		},
		{
			name: "current directory",
			metadata: &ProwMetadata{
				Bucket: "bucket",
				Path:   "path/to/artifacts/123",
			},
			dest: ".",
			want: "gsutil -m cp -r gs://bucket/path/to/artifacts/123/ .",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildGsutilCommand(tt.metadata, tt.dest)
			if got != tt.want {
				t.Errorf("BuildGsutilCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
