package compatibility

import "github.com/compose-spec/compose-go/types"

type Checker interface {
	Check(project *types.Project)
	Errors() []error
}
