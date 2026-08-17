package model

import (
	"encoding/hex"
	"testing"
)

func TestNewID(t *testing.T) {
	id := NewID()
	if len(id) != 32 {
		t.Fatalf("expected 32-char hex, got %d chars: %s", len(id), id)
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("not valid hex: %s", id)
	}
	id2 := NewID()
	if id == id2 {
		t.Fatal("two consecutive IDs should differ")
	}
}

func TestAllPorts(t *testing.T) {
	tests := []struct {
		name     string
		tpl      Template
		expected []int
	}{
		{
			name:     "internal only",
			tpl:      Template{InternalPort: 4096},
			expected: []int{4096},
		},
		{
			name:     "internal + extra deduped",
			tpl:      Template{InternalPort: 4096, ExtraPorts: []int{3000, 5173, 3000}},
			expected: []int{4096, 3000, 5173},
		},
		{
			name:     "zero internal port",
			tpl:      Template{InternalPort: 0, ExtraPorts: []int{3000}},
			expected: []int{3000},
		},
		{
			name:     "negative and zero extra ports filtered",
			tpl:      Template{InternalPort: 4096, ExtraPorts: []int{-1, 0, 8000}},
			expected: []int{4096, 8000},
		},
		{
			name:     "nil extra ports",
			tpl:      Template{InternalPort: 8080},
			expected: []int{8080},
		},
		{
			name:     "all filtered out",
			tpl:      Template{InternalPort: 0, ExtraPorts: []int{-1, 0}},
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tpl.AllPorts()
			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Fatalf("got %v, want %v", got, tt.expected)
				}
			}
		})
	}
}
