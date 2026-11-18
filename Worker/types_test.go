package main

import (
	"testing"
)

func TestTaskPayloadNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    TaskPayload
		expected TaskPayload
	}{
		{
			name:     "Standard Worker Schema (prime)",
			input:    TaskPayload{Type: "prime", Value: 50000},
			expected: TaskPayload{Type: "prime", Value: 50000},
		},
		{
			name:     "README Schema (prime_computation + target)",
			input:    TaskPayload{TaskType: "prime_computation", Target: 25000},
			expected: TaskPayload{Type: "prime", TaskType: "prime_computation", Value: 25000, Target: 25000},
		},
		{
			name:     "README Benchmark Shorthand (only n)",
			input:    TaskPayload{N: 100000},
			expected: TaskPayload{Type: "prime", Value: 100000, N: 100000},
		},
		{
			name:     "Standard Worker Schema (is_power_of_two)",
			input:    TaskPayload{Type: "is_power_of_two", Value: 2048},
			expected: TaskPayload{Type: "is_power_of_two", Value: 2048},
		},
		{
			name:     "Standard Worker Schema (stress_cpu)",
			input:    TaskPayload{Type: "stress_cpu", Value: 10},
			expected: TaskPayload{Type: "stress_cpu", Value: 10},
		},
		{
			name:     "Zero value fallback for prime",
			input:    TaskPayload{Type: "prime", Value: 0},
			expected: TaskPayload{Type: "prime", Value: 50000},
		},
		{
			name:     "Zero value fallback for stress_cpu",
			input:    TaskPayload{Type: "stress_cpu", Value: 0},
			expected: TaskPayload{Type: "stress_cpu", Value: 5},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.Normalize()
			if tc.input.Type != tc.expected.Type {
				t.Errorf("expected Type %q, got %q", tc.expected.Type, tc.input.Type)
			}
			if tc.input.Value != tc.expected.Value {
				t.Errorf("expected Value %d, got %d", tc.expected.Value, tc.input.Value)
			}
		})
	}
}
