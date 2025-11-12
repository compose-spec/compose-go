/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package transform

import (
	"fmt"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// exampleEnhancedPortDefaults is an example of an enhanced port defaults function
// that supports additional protocols and domain names in the published field.
// This function can be used as a replacement for the default portDefaults
// by registering it with RegisterDefaultValue.
//
// Example usage:
//
//	RegisterDefaultValue("services.*.ports.*", exampleEnhancedPortDefaults)
//
// This function supports:
// - Additional protocols: "http", "https", "tcp", "udp"
// - Domain names in the published field (e.g., "example.com:80")
// - All existing functionality of the original portDefaults
func exampleEnhancedPortDefaults(data any, _ tree.Path, _ bool) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		// Set default protocol if not specified
		if _, ok := v["protocol"]; !ok {
			v["protocol"] = "tcp"
		}

		// Set default mode if not specified
		if _, ok := v["mode"]; !ok {
			v["mode"] = "ingress"
		}

		// Enhanced protocol handling with app_protocol
		protocol, _ := v["protocol"].(string)
		switch protocol {
		case "http":
			if _, ok := v["app_protocol"]; !ok {
				v["app_protocol"] = "http1.1"
			}
		case "https":
			if _, ok := v["app_protocol"]; !ok {
				v["app_protocol"] = "http2"
			}
		}

		// Auto-generate port name based on target port
		if _, ok := v["name"]; !ok {
			if target, ok := v["target"].(int); ok {
				switch target {
				case 80:
					v["name"] = "http"
				case 443:
					v["name"] = "https"
				case 3306:
					v["name"] = "mysql"
				case 5432:
					v["name"] = "postgres"
				case 6379:
					v["name"] = "redis"
				case 27017:
					v["name"] = "mongodb"
				default:
					v["name"] = fmt.Sprintf("port-%d", target)
				}
			}
		}

		// Handle domain names in published field
		if published, ok := v["published"].(string); ok {
			// Check if published contains a domain name, very KISS check for this example
			if strings.Contains(published, ".") && strings.Contains(published, ":") {
				parts := strings.SplitN(published, ":", 2)
				if len(parts) == 2 {
					v["x-published-domain"] = parts[0]
					v["x-published-port"] = parts[1]
				}
			}
		}

		// Normalize host_ip shortcuts
		if hostIP, ok := v["host_ip"].(string); ok {
			switch hostIP {
			case "localhost":
				v["host_ip"] = "127.0.0.1"
			case "*":
				v["host_ip"] = "0.0.0.0"
			}
		}

		// Add monitoring metadata for common ports
		if target, ok := v["target"].(int); ok {
			switch target {
			case 80, 443, 8080, 8443:
				v["x-metrics-enabled"] = true
				v["x-metrics-path"] = "/metrics"
			case 9090: // Prometheus
				v["x-metrics-enabled"] = true
				v["x-metrics-type"] = "prometheus"
			}
		}

		return v, nil
	default:
		return data, nil
	}
}

func TestRegisterDefaultValue(t *testing.T) {
	// Save original transformers, so as not to break possible other tests
	originalTransformers := make(map[tree.Path]Func)
	for k, v := range DefaultValues {
		originalTransformers[k] = v
	}
	t.Cleanup(func() {
		DefaultValues = originalTransformers
	})

	// Register the function
	RegisterDefaultValue("services.*.ports.*", exampleEnhancedPortDefaults)

	testCases := []struct {
		name         string
		inputYAML    string
		expectedYAML string
	}{
		{
			name: "basic port with defaults and auto-generated name",
			inputYAML: `
services:
  web:
    ports:
      - target: 80
`,
			expectedYAML: `
services:
  web:
    ports:
      - target: 80
        protocol: tcp
        mode: ingress
        name: http
        x-metrics-enabled: true
        x-metrics-path: /metrics
`,
		},
		{
			name: "port with https protocol and app_protocol",
			inputYAML: `
services:
  web:
    ports:
      - target: 443
        protocol: https
`,
			expectedYAML: `
services:
  web:
    ports:
      - target: 443
        protocol: https
        app_protocol: http2
        mode: ingress
        name: https
        x-metrics-enabled: true
        x-metrics-path: /metrics
`,
		},
		{
			name: "port with domain name in published field",
			inputYAML: `
services:
  web:
    ports:
      - target: 80
        published: "example.com:8080"
`,
			expectedYAML: `
services:
  web:
    ports:
      - target: 80
        published: "example.com:8080"
        protocol: tcp
        mode: ingress
        name: http
        x-published-domain: example.com
        x-published-port: "8080"
        x-metrics-enabled: true
        x-metrics-path: /metrics
`,
		},
		{
			name: "database port with auto-generated name",
			inputYAML: `
services:
  db:
    ports:
      - target: 3306
`,
			expectedYAML: `
services:
  db:
    ports:
      - target: 3306
        protocol: tcp
        mode: ingress
        name: mysql
`,
		},
		{
			name: "host_ip normalization",
			inputYAML: `
services:
  web:
    ports:
      - target: 8080
        host_ip: localhost
      - target: 8081
        host_ip: "*"
`,
			expectedYAML: `
services:
  web:
    ports:
      - target: 8080
        host_ip: "127.0.0.1"
        protocol: tcp
        mode: ingress
        name: port-8080
        x-metrics-enabled: true
        x-metrics-path: /metrics
      - target: 8081
        host_ip: "0.0.0.0"
        protocol: tcp
        mode: ingress
        name: port-8081
`,
		},
		{
			name: "prometheus port with monitoring metadata",
			inputYAML: `
services:
  prometheus:
    ports:
      - target: 9090
`,
			expectedYAML: `
services:
  prometheus:
    ports:
      - target: 9090
        protocol: tcp
        mode: ingress
        name: port-9090
        x-metrics-enabled: true
        x-metrics-type: prometheus
`,
		},
		{
			name: "http protocol with app_protocol",
			inputYAML: `
services:
  web:
    ports:
      - target: 3000
        protocol: http
`,
			expectedYAML: `
services:
  web:
    ports:
      - target: 3000
        protocol: http
        app_protocol: http1.1
        mode: ingress
        name: port-3000
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var input map[string]any
			err := yaml.Unmarshal([]byte(tc.inputYAML), &input)
			assert.NilError(t, err)

			var expected map[string]any
			err = yaml.Unmarshal([]byte(tc.expectedYAML), &expected)
			assert.NilError(t, err)

			result, err := SetDefaultValues(input)
			assert.NilError(t, err)
			assert.DeepEqual(t, result, expected)
		})
	}
}
