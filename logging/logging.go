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

// Package logging defines the contract used to report the warnings emitted
// while loading a Compose model.
package logging

// Logger receives the warnings emitted while a Compose model is loaded:
// obsolete attributes, deprecated syntaxes, unset variables, ambiguous file
// discovery, and so on.
//
// It is satisfied as-is by *logrus.Logger, *logrus.Entry and by the sugared
// logger of go.uber.org/zap, and a log/slog based implementation is a
// one-liner:
//
//	type slogLogger struct{ *slog.Logger }
//
//	func (l slogLogger) Warnf(format string, args ...any) {
//		l.Warn(fmt.Sprintf(format, args...))
//	}
//
// When no Logger is configured, warnings are written through the standard
// logrus logger, which is how compose-go has always reported them.
type Logger interface {
	// Warnf reports a warning, formatted as fmt.Sprintf would.
	Warnf(format string, args ...any)
}
