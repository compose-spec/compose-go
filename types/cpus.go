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

package types

import (
	"fmt"
	"strconv"

	"go.yaml.in/yaml/v4"
)

type NanoCPUs float32

func (n *NanoCPUs) Value() float32 {
	return float32(*n)
}

// UnmarshalYAML accepts a scalar number or numeric string and stores its
// float32 value in n. Mirrors DecodeMapstructure for yaml.v4 native
// decoding.
func (n *NanoCPUs) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected scalar nanocpus, got kind %d", value.Kind)
	}
	f, err := strconv.ParseFloat(value.Value, 64)
	if err != nil {
		return fmt.Errorf("invalid cpus value %q: %w", value.Value, err)
	}
	*n = NanoCPUs(f)
	return nil
}
