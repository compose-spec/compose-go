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
	"github.com/compose-spec/compose-go/v2/format"
	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/pkg/errors"
)

func transformVolumeMount(data any, p tree.Path) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		return v, nil
	case string:
		volume, err := format.ParseVolume(v) // TODO(ndeloof) ParseVolume should not rely on types and return map[string]
		if err != nil {
			return nil, err
		}

		return encode(volume)
	default:
		return data, errors.Errorf("invalid type %T for service volume mount", v)
	}
}
