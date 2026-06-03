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

// Location records the source position of a compose value: the absolute
// path of the file that declared it (or "(inline)" when the document was
// built from in-memory bytes) plus the 1-based line and column emitted
// by the YAML parser. Zero on Line or Column means "not recorded".
type Location struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// Sources maps a dotted compose path (e.g. "services.web.image") to the
// source Location of the corresponding value. It is populated on
// *Project.Sources when the loader was invoked with the Diagnostics
// opt-in.
//
// The map covers every mapping path reachable from the merged tree at
// the time of normalization. Paths under sequences are stable per index
// (e.g. "services.web.ports.0") only when those entries survived the
// canonical transform without re-encoding; downstream consumers should
// treat missing entries as "position not recorded" rather than as an
// error.
type Sources map[string]Location
