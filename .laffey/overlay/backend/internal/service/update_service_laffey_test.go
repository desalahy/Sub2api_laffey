package service

import "testing"

func TestCompareVersionsIncludesLaffeyRevision(t *testing.T) {
	t.Parallel()
	tests := []struct {
		current string
		latest  string
		want    int
	}{
		{current: "0.1.149-laffey.1", latest: "0.1.149-laffey.2", want: -1},
		{current: "v0.1.149-laffey.2", latest: "v0.1.149-laffey.2", want: 0},
		{current: "0.1.150-laffey.1", latest: "0.1.149-laffey.9", want: 1},
	}
	for _, tt := range tests {
		if got := compareVersions(tt.current, tt.latest); got != tt.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.current, tt.latest, got, tt.want)
		}
	}
}

