package generator

import "testing"

func TestFieldNameToTSAccessor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"id", "id"},
		{"user_id", "userId"},
		{"first_name", "firstName"},
		{"a", "a"},
		{"some_long_field_name", "someLongFieldName"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := fieldNameToTSAccessor(tt.input)
			if got != tt.expected {
				t.Errorf("fieldNameToTSAccessor(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
