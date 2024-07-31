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

package utils

import (
	"reflect"
	"testing"
)

func TestCloneMap(t *testing.T) {
	original := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]interface{}{
			"nestedKey1": "nestedValue1",
			"nestedKey2": map[string]interface{}{
				"nestedNestedKey1": "nestedNestedValue1",
			},
		},
		"key3": 42,
	}

	clone := CloneMap(original)

	// Check if the clone is deeply equal to the original
	if !reflect.DeepEqual(original, clone) {
		t.Errorf("Clone is not equal to the original. Expected %v, got %v", original, clone)
	}

	// Modify the clone and check if original is unchanged (deep clone verification)
	clone["key2"].(map[string]interface{})["nestedKey1"] = "modifiedValue"
	if reflect.DeepEqual(original, clone) {
		t.Errorf("Original was changed when modifying the clone. Original: %v, Clone: %v", original, clone)
	}

	clone["key2"].(map[string]interface{})["nestedKey2"].(map[string]interface{})["nestedNestedKey1"] = "modifiedNestedNestedValue"
	if reflect.DeepEqual(original, clone) {
		t.Errorf("Original was changed when modifying the nested clone. Original: %v, Clone: %v", original, clone)
	}
}
