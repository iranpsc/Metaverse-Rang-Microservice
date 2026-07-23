package handler

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// parseIndexedQueryArray collects values for keys like name[0], name[1], ...
func parseIndexedQueryArray(query url.Values, name string) []string {
	prefix := name + "["
	type indexedValue struct {
		index int
		value string
	}

	var items []indexedValue
	for key, vals := range query {
		if len(vals) == 0 || !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, "]") {
			continue
		}
		indexStr := key[len(prefix) : len(key)-1]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			continue
		}
		items = append(items, indexedValue{index: index, value: vals[0]})
	}

	if len(items) == 0 {
		return nil
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].index < items[j].index
	})

	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.value
	}
	return out
}
