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

package format

import "github.com/compose-spec/compose-go/v2/types"

// ShortSyntax renders a volume in the short syntax, `[SOURCE:]TARGET[:OPTIONS]`.
// It reports false when the short syntax can't say what the volume says.
// See types.ServiceVolumeConfig.ShortSyntax, which it only calls: the check lives
// next to the parser it relies on.
func ShortSyntax(v types.ServiceVolumeConfig) (string, bool) {
	return v.ShortSyntax()
}

// IsShortSyntaxExpressible tells whether the short syntax can say what the volume says.
// See types.ServiceVolumeConfig.IsShortSyntaxExpressible.
func IsShortSyntaxExpressible(v types.ServiceVolumeConfig) bool {
	return v.IsShortSyntaxExpressible()
}
