package main

import (
	"errors"
	"fmt"
	"strings"
)

func buildErrorMessageForMetrics(err error, settings RequestMetricSettings) error {
	settingLine := []string{}

	if settings.Name != "" {
		settingLine = append(
			settingLine,
			fmt.Sprintf("name[%v]", settings.Name),
		)
	}

	if settings.Filter != "" {
		settingLine = append(
			settingLine,
			fmt.Sprintf("filter[%v]", settings.Filter),
		)
	}

	if len(settingLine) >= 1 {
		err = errors.New(fmt.Sprintf("%v: %v", strings.Join(settingLine, " "), err))
	}

	return err
}
