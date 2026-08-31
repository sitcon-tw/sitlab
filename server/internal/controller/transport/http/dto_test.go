package httpserver

import (
	"testing"

	"example.com/project-template/internal/domain/board"
)

func TestMapCard(t *testing.T) {
	tests := []struct {
		name       string
		statusName string
		want       *string
	}{
		{
			name:       "exposes granular GitLab status",
			statusName: "Duplicate",
			want:       stringPointer("Duplicate"),
		},
		{
			name: "uses null when GitLab has no status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := mapCard(board.Card{GitLabStatusName: tt.statusName})
			if response.GitLabStatusName == nil && tt.want == nil {
				return
			}
			if response.GitLabStatusName == nil || tt.want == nil || *response.GitLabStatusName != *tt.want {
				t.Fatalf("GitLabStatusName = %v, want %v", response.GitLabStatusName, tt.want)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
