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
	"math"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/errdefs"
	"github.com/compose-spec/compose-go/v2/types"
)

// restartPolicyName maps the name of a restart policy, as written in `restart`
// or in `deploy.restart_policy.condition`, to the vocabulary of `restart`.
// The second result is false for a name Compose does not know, which is left
// for the platform to report.
func restartPolicyName(name string) (string, bool) {
	switch name {
	case "no", "none":
		return types.RestartPolicyNo, true
	case "always", "any":
		return types.RestartPolicyAlways, true
	case types.RestartPolicyOnFailure, types.RestartPolicyUnlessStopped:
		return name, true
	}
	return "", false
}

// mergeRestart merges `deploy.restart_policy` into `restart`: the two are
// spellings of one policy, where `condition` is the name of the policy and
// `max_attempts` its retry count (`restart: on-failure:3`), while `delay` and
// `window` have no counterpart in `restart`. Each attribute may thus complete
// the other, but where both say something it must be the same thing. An
// attribute that is not set does not say anything: a `restart_policy` made of
// a `delay` alone leaves `restart` as it is.
//
// It returns the value `restart` takes, or "" when there is no policy to carry
// (nothing is set, or a value is left for the platform to report), and the
// attribute that contradicts `restart`, if any, in which case nothing is
// carried.
func mergeRestart(restart, condition string, maxAttempts *uint64) (merged, conflict string) {
	var name string
	restartName, restartCount, hasRestartCount := strings.Cut(restart, ":")
	if restart != "" {
		var known bool
		if name, known = restartPolicyName(restartName); !known {
			return "", ""
		}
	}
	retries, hasRetries := parseRetries(restartCount, hasRestartCount)
	if hasRestartCount && !hasRetries {
		// `on-failure:abc`: left for the platform to report
		return "", ""
	}

	if condition != "" {
		conditionName, known := restartPolicyName(condition)
		if !known {
			return "", ""
		}
		if name != "" && name != conditionName {
			return "", "deploy.restart_policy.condition"
		}
		name = conditionName
	}

	// zero retries is no limit, as for `on-failure:0`: it neither contradicts a
	// policy that has no retry count nor gives one to carry
	if maxAttempts != nil && (*maxAttempts != 0 || name == types.RestartPolicyOnFailure) {
		// `restart` already pins a policy: only `on-failure` has a retry count
		// to be completed or contradicted by `max_attempts`
		if restart != "" && (name != types.RestartPolicyOnFailure || hasRetries && retries != *maxAttempts) {
			return "", "deploy.restart_policy.max_attempts"
		}
		if restart == "" && name != "" && name != types.RestartPolicyOnFailure {
			// a retry count with this policy is no `restart` value: the
			// platform reports it, as it does today
			return "", ""
		}
		retries, hasRetries = *maxAttempts, true
	}

	if name == "" {
		// nothing but `delay`, `window` or `max_attempts`: no policy to carry
		return "", ""
	}
	if hasRetries {
		name += ":" + strconv.FormatUint(retries, 10)
	}
	return name, ""
}

// carryRestartPolicy writes the outcome of mergeRestart in the `restart` of a
// service of the canonical model. It runs when the model is normalized, that is
// once the files, `include` and `extends` are merged (either attribute may be
// changed until then), and a service whose two attributes contradict each other
// is left as it is, to be rejected by checkRestartPolicy, which the consistency
// check can be told to skip. A value that has not the shape the schema gives it
// is left to be reported where the model is validated or bound.
func carryRestartPolicy(service map[string]any) {
	deploy, _ := service["deploy"].(map[string]any)
	policy, _ := deploy["restart_policy"].(map[string]any)
	if policy == nil {
		return
	}
	maxAttempts, ok := attempts(policy["max_attempts"])
	if !ok {
		// a retry count which is not one: carrying the policy without it would
		// lose it in silence, so it is left to be reported when the model is bound
		return
	}
	restart, _ := service["restart"].(string)
	condition, _ := policy["condition"].(string)
	if merged, conflict := mergeRestart(restart, condition, maxAttempts); conflict == "" && merged != "" {
		service["restart"] = merged
	}
}

// attempts reads the `max_attempts` of a model: nil when it is not set, and
// false when it is set to something which is not a retry count.
func attempts(value any) (*uint64, bool) {
	var n uint64
	switch v := value.(type) {
	case nil:
		return nil, true
	case int:
		if v < 0 {
			return nil, false
		}
		n = uint64(v)
	case int64:
		if v < 0 {
			return nil, false
		}
		n = uint64(v)
	case uint64:
		n = v
	case float64:
		if v < 0 || v != math.Trunc(v) || v >= math.Exp2(64) {
			return nil, false
		}
		n = uint64(v)
	case string:
		parsed, ok := parseRetries(v, true)
		if !ok {
			return nil, false
		}
		n = parsed
	default:
		return nil, false
	}
	return &n, true
}

// checkRestartPolicy rejects a service whose `restart` and
// `deploy.restart_policy` contradict each other, as it does for `scale` and
// `deploy.replicas`: Compose does not pick one of two contradictory policies
// in silence.
func checkRestartPolicy(s types.ServiceConfig) error {
	if s.Deploy == nil || s.Deploy.RestartPolicy == nil {
		return nil
	}
	policy := s.Deploy.RestartPolicy
	if _, conflict := mergeRestart(s.Restart, policy.Condition, policy.MaxAttempts); conflict != "" {
		return fmt.Errorf("services.%s: can't set distinct values on 'restart' and '%s': %w", s.Name, conflict, errdefs.ErrInvalid)
	}
	return nil
}

// parseRetries reads a retry count. It reports false for an absent or
// malformed count, which is left for the platform to report.
func parseRetries(value string, present bool) (uint64, bool) {
	if !present {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 10, 64)
	return n, err == nil
}
