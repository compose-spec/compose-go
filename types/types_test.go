package types

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParsePortConfig(t *testing.T) {
	testCases := []struct {
		value         string
		expectedError string
		expected      []ServicePortConfig
	}{
		{
			value: "80",
			expected: []ServicePortConfig{
				{
					Protocol: "tcp",
					Target:   80,
					Mode:     "ingress",
				},
			},
		},
		{
			value: "80:8080",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "8080:80/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    80,
					Published: 8080,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80:8080/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-81:8080-8081/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
				{
					Protocol:  "tcp",
					Target:    8081,
					Published: 81,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-82:8080-8082/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8081,
					Published: 81,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8082,
					Published: 82,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-82:8080/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 81,
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8080,
					Published: 82,
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-80:8080/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
		{
			value:         "9999999",
			expectedError: "Invalid containerPort: 9999999",
		},
		{
			value:         "80/xyz",
			expectedError: "Invalid proto: xyz",
		},
		{
			value:         "tcp",
			expectedError: "Invalid containerPort: tcp",
		},
		{
			value:         "udp",
			expectedError: "Invalid containerPort: udp",
		},
		{
			value:         "",
			expectedError: "No port specified: <empty>",
		},
		{
			value: "1.1.1.1:80:80",
			expected: []ServicePortConfig{
				{
					HostIP:    "1.1.1.1",
					Protocol:  "tcp",
					Target:    80,
					Published: 80,
					Mode:      "ingress",
				},
			},
		},
	}
	for _, tc := range testCases {
		ports, err := ParsePortConfig(tc.value)
		if tc.expectedError != "" {
			assert.Error(t, err, tc.expectedError)
			continue
		}
		assert.NilError(t, err)
		assert.Check(t, is.Len(ports, len(tc.expected)))
		for _, expectedPortConfig := range tc.expected {
			assertContains(t, ports, expectedPortConfig)
		}
	}
}

func assertContains(t *testing.T, portConfigs []ServicePortConfig, expected ServicePortConfig) {
	var contains = false
	for _, portConfig := range portConfigs {
		if portConfig == expected {
			contains = true
			break
		}
	}
	if !contains {
		t.Errorf("expected %v to contain %v, did not", portConfigs, expected)
	}
}

func TestSet(t *testing.T) {
	s := make(set)
	s.append("one")
	s.append("two")
	s.append("three")
	s.append("two")
	assert.Equal(t, len(s.toSlice()), 3)
}
