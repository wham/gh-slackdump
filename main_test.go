package main

import (
	"testing"
	"time"
)

func TestExtractWorkspaceURL(t *testing.T) {
	tests := []struct {
		name    string
		link    string
		want    string
		wantErr bool
	}{
		{
			name: "regular workspace channel link",
			link: "https://myworkspace.slack.com/archives/C09036MGFJ4",
			want: "https://myworkspace.slack.com",
		},
		{
			name: "enterprise workspace channel link",
			link: "https://myworkspace.enterprise.slack.com/archives/CMH59UX4P",
			want: "https://myworkspace.enterprise.slack.com",
		},
		{
			name: "thread link",
			link: "https://myworkspace.slack.com/archives/C09036MGFJ4/p1771747003176409",
			want: "https://myworkspace.slack.com",
		},
		{
			name:    "non-slack domain",
			link:    "https://example.com/archives/C09036MGFJ4",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			link:    "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractWorkspaceURL(tt.link)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractWorkspaceURL(%q) error = %v, wantErr %v", tt.link, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractWorkspaceURL(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version should not be empty")
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "empty string returns zero time",
			input: "",
			want:  time.Time{},
		},
		{
			name:  "RFC3339 timestamp",
			input: "2024-01-15T10:30:00Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "date-only YYYY-MM-DD",
			input: "2024-01-15",
			want:  time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid format",
			input:   "Jan 15 2024",
			wantErr: true,
		},
		{
			name:    "partial date",
			input:   "2024-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
