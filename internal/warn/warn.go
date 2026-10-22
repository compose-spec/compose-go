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

// Package warn dispatches a warning to the configured logging.Logger, or to
// the standard logrus logger when none is configured.
package warn

import (
	"github.com/compose-spec/compose-go/v2/logging"
	"github.com/sirupsen/logrus"
)

// Warnf reports a warning to l, falling back to the standard logrus logger
// when l is nil so that the default behavior is left untouched.
func Warnf(l logging.Logger, format string, args ...any) {
	if l == nil {
		logrus.Warnf(format, args...)
		return
	}
	l.Warnf(format, args...)
}
