package loader

import (
	"fmt"
	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/types"
	"github.com/pkg/errors"
)

// validate a compose model is consistent
func validate(project *types.Project) error {
	for _, s := range project.Services {
		for network := range s.Networks {
			if _, ok := project.Networks[network]; !ok {
				return errors.Wrap(errdefs.ErrInvalid, fmt.Sprintf("service %q refers to undefined network %s", s.Name, network))
			}
		}
		for _, volume := range s.Volumes {
			if _, ok := project.Volumes[volume.Source]; !ok {
				return errors.Wrap(errdefs.ErrInvalid, fmt.Sprintf("service %q refers to undefined volume %s", s.Name, volume))
			}
		}
		for _, secret := range s.Secrets {
			if _, ok := project.Secrets[secret.Source]; !ok {
				return errors.Wrap(errdefs.ErrInvalid, fmt.Sprintf("service %q refers to undefined secret %s", s.Name, secret))
			}
		}
		for _, config := range s.Configs {
			if _, ok := project.Configs[config.Source]; !ok {
				return errors.Wrap(errdefs.ErrInvalid, fmt.Sprintf("service %q refers to undefined config %s", s.Name, config))
			}
		}
	}
}
