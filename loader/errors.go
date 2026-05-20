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

package loader

import (
	"fmt"

	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// nodeErrf returns an error whose message is prefixed with the origin of the
// given yaml node, formatted as "<source>:<line>:<column>: <message>". When
// ctx is nil only the node's position is reported; when node is nil only the
// source file is reported.
func nodeErrf(ctx *types.NodeContext, node *yaml.Node, format string, args ...any) error {
	origin := ctx.OriginAt(node)
	prefix := origin.String()
	msg := fmt.Sprintf(format, args...)
	if prefix == "" {
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("%s: %s", prefix, msg)
}

// wrapNodeErr wraps err with the origin of the given yaml node so that the
// underlying error chain remains traversable with errors.Is / errors.As.
func wrapNodeErr(ctx *types.NodeContext, node *yaml.Node, err error) error {
	if err == nil {
		return nil
	}
	prefix := ctx.OriginAt(node).String()
	if prefix == "" {
		return err
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
