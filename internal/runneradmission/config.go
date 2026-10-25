// Package runneradmission enforces the Hostinger runner trust domains before jobs run.
package runneradmission

import (
	"errors"
	"strings"

	"github.com/caarlos0/env/v11"
)

//go:generate go tool envdoc -output environment.md -format markdown
type Config struct {
	// Repository selected by the GitHub runner.
	Repository string `env:"GITHUB_REPOSITORY,required,notEmpty"`
	// Actor selected by the GitHub runner.
	Actor string `env:"GITHUB_ACTOR,required,notEmpty"`
	// GitHub event name. Only explicitly supported events are admitted.
	Event string `env:"GITHUB_EVENT_NAME,required,notEmpty"`
	// Git ref selected by the GitHub runner.
	Ref string `env:"GITHUB_REF,required,notEmpty"`
	// Runner-provided event JSON file.
	EventPath string `env:"GITHUB_EVENT_PATH,required,notEmpty"`
}

func Parse(values map[string]string) (Config, error) {
	c, err := env.ParseAsWithOptions[Config](env.Options{Environment: values})
	if err != nil {
		return Config{}, errors.New("missing runner context")
	}
	for _, value := range []string{c.Repository, c.Actor, c.Event, c.Ref, c.EventPath} {
		if strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") {
			return Config{}, errors.New("invalid runner context")
		}
	}
	return c, nil
}

func Load() (Config, error) { return Parse(nil) }
