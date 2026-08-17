package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestMatchAsset(t *testing.T) {
	cases := []struct {
		name     string
		req      updater.CheckRequest
		assets   []github.ReleaseAsset
		expected int
	}{
		{
			name:   "linux selects raw macros",
			req:    updater.CheckRequest{Platform: "linux", Arch: "amd64"},
			assets: []github.ReleaseAsset{{Name: "macros.exe"}, {Name: "macros.AppImage"}, {Name: "macros"}},
			expected: 2,
		},
		{
			name:   "windows selects macros.exe",
			req:    updater.CheckRequest{Platform: "windows", Arch: "amd64"},
			assets: []github.ReleaseAsset{{Name: "macros.exe"}, {Name: "macros.AppImage"}, {Name: "macros"}},
			expected: 0,
		},
		{
			name:     "linux missing raw asset returns -1",
			req:      updater.CheckRequest{Platform: "linux", Arch: "amd64"},
			assets:   []github.ReleaseAsset{{Name: "macros.AppImage"}, {Name: "macros.exe"}},
			expected: -1,
		},
		{
			name:     "windows missing exe returns -1",
			req:      updater.CheckRequest{Platform: "windows", Arch: "amd64"},
			assets:   []github.ReleaseAsset{{Name: "macros"}, {Name: "macros.AppImage"}},
			expected: -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchAsset(tc.req, tc.assets); got != tc.expected {
				t.Fatalf("MatchAsset() = %d, want %d", got, tc.expected)
			}
		})
	}
}
