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
	"reflect"

	"github.com/sirupsen/logrus"
)

var null = reflect.ValueOf("x-null")

func applyNullOverrides(i interface{}) error {
	return _applyNullOverrides(reflect.ValueOf(i))
}

func _applyNullOverrides(val reflect.Value) error {
	val = reflect.Indirect(val)
	typ := val.Type()
	if typ.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < typ.NumField(); i++ {
		v := reflect.Indirect(val.Field(i))

		name := typ.Field(i).Name
		switch v.Kind() {
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				err := _applyNullOverrides(v.Index(i))
				if err != nil {
					return err
				}
			}
		case reflect.Struct:
			// search for Extensions["x-null"]. If set, reset field
			extensions := v.FieldByName("Extensions")
			if extensions.IsValid() {
				xNull := extensions.MapIndex(null)
				if xNull.IsValid() {
					logrus.Debugf("%s reset to null", name)
					f := val.Field(i)
					if !f.CanSet() {
						return fmt.Errorf("can't override attribute %s", name)
					}
					// f.SetZero() requires go 1.20
					f.Set(reflect.Zero(f.Type()))
					continue
				}
			}
			err := _applyNullOverrides(val.Field(i))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
