package main

import "testing"

func TestClassifyRepoStatus(t *testing.T) {
	tests := []struct {
		name            string
		remoteAhead     bool
		localAhead      bool
		hasLocalChanges bool
		want            string
	}{
		{
			name:        "cloud is newer",
			remoteAhead: true,
			want:        RepoStatusCloudNewer,
		},
		{
			name:       "local branch is newer",
			localAhead: true,
			want:       RepoStatusLocalNewer,
		},
		{
			name:            "local files have uncommitted changes",
			hasLocalChanges: true,
			want:            RepoStatusLocalNewer,
		},
		{
			name:        "both branches have changes",
			remoteAhead: true,
			localAhead:  true,
			want:        RepoStatusConflict,
		},
		{
			name:            "cloud and local files have changes",
			remoteAhead:     true,
			hasLocalChanges: true,
			want:            RepoStatusConflict,
		},
		{
			name: "both are up to date",
			want: RepoStatusSynced,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRepoStatus(tt.remoteAhead, tt.localAhead, tt.hasLocalChanges)
			if got != tt.want {
				t.Errorf("classifyRepoStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
