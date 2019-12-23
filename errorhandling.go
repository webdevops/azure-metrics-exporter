package main

import (
	"errors"
	"fmt"
	"strings"
)

func buildErrorMessageForMetrics(err error, settings RequestMetricSettings) error {
	settingLine := []string{
		fmt.Sprintf("name[%v]", settings.Name),
		fmt.Sprintf("filter[%v]", settings.Filter),
	}
	return errors.New(fmt.Sprintf("%v: %v", strings.Join(settingLine, " "), err))
}
