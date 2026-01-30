package api

import (
	"regexp"
	"strings"
)

type labelFilter struct {
	key   string
	value string
}

func compileRegex(raw string) *regexp.Regexp {
	if raw == "" {
		return regexp.MustCompile(".*")
	}
	compiled, err := regexp.Compile(raw)
	if err != nil {
		return regexp.MustCompile(".*")
	}
	return compiled
}

func parseLabelFilters(items []string) []labelFilter {
	filters := make([]labelFilter, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		filter := labelFilter{key: strings.TrimSpace(parts[0])}
		if len(parts) == 2 {
			filter.value = strings.TrimSpace(parts[1])
		}
		if filter.key != "" {
			filters = append(filters, filter)
		}
	}
	return filters
}

func matchesExcluded(labels map[string]string, filters []labelFilter) bool {
	if len(filters) == 0 || len(labels) == 0 {
		return false
	}
	for _, filter := range filters {
		if value, ok := labels[filter.key]; ok {
			if filter.value == "" || value == filter.value {
				return true
			}
		}
	}
	return false
}
