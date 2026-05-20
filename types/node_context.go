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

// NodeContext captures the loading context of a yaml node parsed from a
// Compose file. It records where the node came from, which directory should
// be used to resolve its relative paths, and which environment variables
// were in scope when the node was parsed.
//
// NodeContext is treated as immutable once attached to a node. The loader
// keeps a map from *yaml.Node to *NodeContext so that information survives
// merging of multiple files.
//
// Parent is non-nil for contexts produced by an include directive and points
// to the context of the file that declared the include. It allows lookups
// to fall back to the enclosing scope.
type NodeContext struct {
	// Source is the yaml file the node was parsed from.
	Source string
	// WorkingDir is the base directory to resolve relative paths declared
	// in nodes attached to this context.
	WorkingDir string
	// Env is the environment variables available for interpolation in this
	// context. For an included file, this typically includes the variables
	// supplied by include.env_file in addition to the parent environment.
	Env Mapping
	// Parent is the enclosing context (nil at the root of a load).
	Parent *NodeContext
}

// Origin pairs a NodeContext with a position inside the source file. It is
// returned by diagnostic APIs (Project.OriginOf, Project.Origins) to point
// back to the location where a value was defined.
type Origin struct {
	// Source is the yaml file path.
	Source string
	// Line is the 1-based line number in Source.
	Line int
	// Column is the 1-based column number in Source.
	Column int
}
