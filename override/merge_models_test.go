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

package override

import (
	"testing"
)

func Test_mergeYamlServiceModelsShortSyntax(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    models:
      - llm
      - embedding-model
`, `
services:
  test:
    models:
      - vision-model
`, `
services:
  test:
    image: foo
    models:
      llm:
      embedding-model:
      vision-model:
`)
}

func Test_mergeYamlServiceModelsLongSyntax(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    models:
      llm:
        endpoint_var: AI_MODEL_URL
        model_var: AI_MODEL_NAME
`, `
services:
  test:
    models:
      embedding-model:
        endpoint_var: EMBEDDING_URL
        model_var: EMBEDDING_MODEL
`, `
services:
  test:
    image: foo
    models:
      llm:
        endpoint_var: AI_MODEL_URL
        model_var: AI_MODEL_NAME
      embedding-model:
        endpoint_var: EMBEDDING_URL
        model_var: EMBEDDING_MODEL
`)
}

func Test_mergeYamlServiceModelsMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    models:
      - llm
      - embedding-model
`, `
services:
  test:
    models:
      vision-model:
        endpoint_var: VISION_URL
        model_var: VISION_MODEL
`, `
services:
  test:
    image: foo
    models:
      llm:
      embedding-model:
      vision-model:
        endpoint_var: VISION_URL
        model_var: VISION_MODEL
`)
}

func Test_mergeYamlServiceModelsOverride(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    models:
      llm:
        endpoint_var: OLD_MODEL_URL
        model_var: OLD_MODEL_NAME
`, `
services:
  test:
    models:
      llm:
        endpoint_var: NEW_MODEL_URL
        model_var: NEW_MODEL_NAME
`, `
services:
  test:
    image: foo
    models:
      llm:
        endpoint_var: NEW_MODEL_URL
        model_var: NEW_MODEL_NAME
`)
}

func Test_mergeYamlTopLevelModels(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
models:
  llm:
    model: ai/smollm2
    context_size: 2048
    runtime_flags:
      - "--gpu"
`, `
services:
  test:
    image: foo
models:
  embedding-model:
    model: ai/all-minilm
    context_size: 512
    runtime_flags:
      - "--cpu"
`, `
services:
  test:
    image: foo
models:
  llm:
    model: ai/smollm2
    context_size: 2048
    runtime_flags:
      - "--gpu"
  embedding-model:
    model: ai/all-minilm
    context_size: 512
    runtime_flags:
      - "--cpu"
`)
}

func Test_mergeYamlModelsCompleteScenario(t *testing.T) {
	assertMergeYaml(t, `
services:
  app:
    image: myapp
    models:
      - llm
  worker:
    image: worker
    models:
      embedding-model:
        endpoint_var: EMBEDDING_URL
models:
  llm:
    model: ai/smollm2
    context_size: 2048
  embedding-model:
    model: ai/all-minilm
    context_size: 512
`, `
services:
  app:
    models:
      - vision-model
  worker:
    models:
      llm:
        endpoint_var: LLM_URL
        model_var: LLM_NAME
models:
  vision-model:
    model: ai/clip
    context_size: 1024
  llm:
    model: ai/gpt-4
    context_size: 8192
`, `
services:
  app:
    image: myapp
    models:
      llm:
      vision-model:
  worker:
    image: worker
    models:
      embedding-model:
        endpoint_var: EMBEDDING_URL
      llm:
        endpoint_var: LLM_URL
        model_var: LLM_NAME
models:
  llm:
    model: ai/gpt-4
    context_size: 8192
  embedding-model:
    model: ai/all-minilm
    context_size: 512
  vision-model:
    model: ai/clip
    context_size: 1024
`)
}

/*
func Test_mergeYamlModelsRuntimeFlagsMerge(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
models:
  llm:
    model: ai/smollm2
    runtime_flags:
      - "--gpu"
      - "--batch-size=32"
`, `
services:
  test:
    image: foo
models:
  llm:
    model: ai/smollm2
    runtime_flags:
      - "--fp16"
      - "--batch-size=64"
`, `
services:
  test:
    image: foo
models:
  llm:
    model: ai/smollm2
    runtime_flags:
      - "--fp16"
      - "--batch-size=64"
`)
}
*/

func Test_mergeYamlModelsMultipleServices(t *testing.T) {
	assertMergeYaml(t, `
services:
  go-genai:
    models:
      - llm
models:
  llm:
    model: ai/smollm2
    context_size: 2048
`, `
services:
  node-genai:
    models:
      - llm
models:
  llm:
    model: ai/smollm2
    context_size: 2048
`, `
services:
  go-genai:
    models:
      - llm
  node-genai:
    models:
      - llm
models:
  llm:
    model: ai/smollm2
    context_size: 2048
`)
}
