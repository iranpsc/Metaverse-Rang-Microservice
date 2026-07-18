package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type discoveredEndpoint struct {
	Method      string
	Path        string
	Summary     string
	PathParams  []paramDef
	QueryParams []paramDef
}

var (
	handlesCommentRegex = regexp.MustCompile(`^//\s+(.+?)\s+handles\s+(.+)$`)
	pathParamComment    = regexp.MustCompile(`^//\s*Path param:\s*(\w+)\s*(?:\(([^)]+)\))?`)
	queryParamsComment  = regexp.MustCompile(`^//\s*Query params?:\s*(.+)$`)
	queryGetRegex       = regexp.MustCompile(`(?:r\.URL\.Query\(\)|query)\.Get\("([^"]+)"\)`)
	formValueRegex      = regexp.MustCompile(`r\.FormValue\("([^"]+)"\)`)
	openAPIPathRegex    = regexp.MustCompile(`\{([^}:]+)(?::[^}]+)?\}`)
)

func discoverHandlerEndpoints(handlerDir string) (map[string]discoveredEndpoint, error) {
	entries, err := os.ReadDir(handlerDir)
	if err != nil {
		return nil, fmt.Errorf("read handler dir: %w", err)
	}

	byKey := map[string]discoveredEndpoint{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(handlerDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		for _, ep := range parseHandlerFile(string(content)) {
			for _, method := range ep.Method {
				key := endpointKey(method, ep.Path)
				existing, ok := byKey[key]
				if !ok {
					byKey[key] = discoveredEndpoint{
						Method:      method,
						Path:        ep.Path,
						Summary:     ep.Summary,
						PathParams:  ep.PathParams,
						QueryParams: ep.QueryParams,
					}
					continue
				}
				existing.PathParams = mergeParamDefs(existing.PathParams, ep.PathParams)
				existing.QueryParams = mergeParamDefs(existing.QueryParams, ep.QueryParams)
				if existing.Summary == "" {
					existing.Summary = ep.Summary
				}
				byKey[key] = existing
			}
		}
	}
	return byKey, nil
}

type parsedEndpoint struct {
	Method      []string
	Path        string
	Summary     string
	PathParams  []paramDef
	QueryParams []paramDef
}

func parseHandlerFile(content string) []parsedEndpoint {
	lines := strings.Split(content, "\n")
	var endpoints []parsedEndpoint

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		match := handlesCommentRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		summary := strings.TrimSpace(match[1])
		routePart := strings.TrimSpace(match[2])
		methods, path, ok := parseMethodsAndPath(routePart)
		if !ok {
			continue
		}

		ep := parsedEndpoint{
			Method:  methods,
			Path:    normalizeOpenAPIPath(path),
			Summary: summary,
		}

		commentEnd := i
		for j := i + 1; j < len(lines) && j <= i+6; j++ {
			comment := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(comment, "//") {
				commentEnd = j - 1
				break
			}
			if pm := pathParamComment.FindStringSubmatch(comment); pm != nil {
				ep.PathParams = append(ep.PathParams, paramDef{
					Name:        pm[1],
					In:          "path",
					Type:        inferPathParamType(pm[1]),
					Description: strings.TrimSpace(pm[2]),
				})
			}
			if qm := queryParamsComment.FindStringSubmatch(comment); qm != nil {
				ep.QueryParams = append(ep.QueryParams, parseQueryParamsComment(qm[1])...)
			}
			commentEnd = j
		}

		body := extractFunctionBody(lines, commentEnd+1)
		ep.QueryParams = mergeParamDefs(ep.QueryParams, discoverQueryParamsFromBody(body))
		ep.PathParams = mergeParamDefs(ep.PathParams, pathParamsFromOpenAPIPath(ep.Path))

		endpoints = append(endpoints, ep)
		i = commentEnd
	}

	return endpoints
}

func parseMethodsAndPath(routePart string) ([]string, string, bool) {
	parts := strings.Fields(routePart)
	if len(parts) < 2 {
		return nil, "", false
	}

	pathIndex := -1
	for i, part := range parts {
		if strings.HasPrefix(part, "/api") {
			pathIndex = i
			break
		}
	}
	if pathIndex == -1 {
		return nil, "", false
	}

	methodsPart := strings.Join(parts[:pathIndex], " ")
	path := parts[pathIndex]
	if pathIndex+1 < len(parts) {
		path = strings.Join(parts[pathIndex:], " ")
		path = strings.Fields(path)[0]
	}
	// Handler comments are full sentences ("handles GET /api/foo."), so strip
	// trailing sentence punctuation that is not part of the URL path.
	path = strings.TrimRight(path, ".,;:!")

	var methods []string
	for _, chunk := range strings.Fields(methodsPart) {
		for _, method := range strings.Split(chunk, "/") {
			upper := strings.ToUpper(strings.TrimSpace(method))
			if isHTTPMethod(upper) {
				methods = append(methods, strings.ToLower(upper))
			}
		}
	}

	methods = uniqueStrings(methods)
	if len(methods) == 0 {
		return nil, "", false
	}
	return methods, path, true
}

func isHTTPMethod(value string) bool {
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func parseQueryParamsComment(comment string) []paramDef {
	chunks := strings.Split(comment, ",")
	params := make([]paramDef, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		name := chunk
		paramType := "string"
		if idx := strings.Index(chunk, "("); idx > 0 && strings.HasSuffix(chunk, ")") {
			name = strings.TrimSpace(chunk[:idx])
			typeHint := strings.TrimSpace(chunk[idx+1 : len(chunk)-1])
			paramType = queryTypeFromHint(typeHint)
		}
		required := isRequiredQueryParam(name)
		params = append(params, paramDef{
			Name:     name,
			In:       "query",
			Type:     paramType,
			Required: boolPtr(required),
		})
	}
	return params
}

func discoverQueryParamsFromBody(body string) []paramDef {
	seen := map[string]struct{}{}
	var params []paramDef

	for _, name := range queryGetRegex.FindAllStringSubmatch(body, -1) {
		if _, ok := seen[name[1]]; ok {
			continue
		}
		seen[name[1]] = struct{}{}
		params = append(params, paramDef{
			Name:     name[1],
			In:       "query",
			Type:     inferQueryParamType(name[1]),
			Required: boolPtr(isRequiredQueryParam(name[1])),
		})
	}

	for _, name := range formValueRegex.FindAllStringSubmatch(body, -1) {
		if _, ok := seen[name[1]]; ok {
			continue
		}
		seen[name[1]] = struct{}{}
		params = append(params, paramDef{
			Name: name[1],
			In:   "query",
			Type: inferQueryParamType(name[1]),
		})
	}

	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})
	return params
}

func extractFunctionBody(lines []string, start int) string {
	if start >= len(lines) {
		return ""
	}

	funcLine := strings.TrimSpace(lines[start])
	if !strings.HasPrefix(funcLine, "func ") {
		return ""
	}

	var body strings.Builder
	braceDepth := 0
	started := false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			switch ch {
			case '{':
				braceDepth++
				started = true
			case '}':
				braceDepth--
			}
		}
		if started {
			body.WriteString(line)
			body.WriteByte('\n')
		}
		if started && braceDepth == 0 {
			break
		}
	}
	return body.String()
}

func normalizeOpenAPIPath(path string) string {
	return openAPIPathRegex.ReplaceAllString(path, "{$1}")
}

func pathParamsFromOpenAPIPath(path string) []paramDef {
	names := extractPathParamNames(path)
	params := make([]paramDef, 0, len(names))
	for _, name := range names {
		params = append(params, paramDef{
			Name:        name,
			In:          "path",
			Type:        inferPathParamType(name),
			Description: pathParamDescription(name),
		})
	}
	return params
}

func endpointKey(method, path string) string {
	return strings.ToUpper(method) + ":" + pathPattern(path)
}

func pathPattern(path string) string {
	return openAPIPathRegex.ReplaceAllString(path, "{}")
}

func lookupDiscovered(discovered map[string]discoveredEndpoint, method, path string) (discoveredEndpoint, bool) {
	key := endpointKey(method, path)
	if ep, ok := discovered[key]; ok {
		return ep, true
	}
	return discoveredEndpoint{}, false
}

func mergeDiscoveredPathParams(routePath string, discovered []paramDef, overrides map[string]paramDef) []paramDef {
	routeNames := extractPathParamNames(routePath)
	if len(routeNames) == 0 {
		return nil
	}

	discoveredByName := map[string]paramDef{}
	for _, p := range discovered {
		discoveredByName[p.Name] = p
	}

	params := make([]paramDef, 0, len(routeNames))
	for i, routeName := range routeNames {
		def := paramDef{
			Name:        routeName,
			In:          "path",
			Type:        inferPathParamType(routeName),
			Description: pathParamDescription(routeName),
		}
		if i < len(discovered) {
			def.Type = discovered[i].Type
			if discovered[i].Description != "" {
				def.Description = discovered[i].Description
			}
			if discovered[i].Format != "" {
				def.Format = discovered[i].Format
			}
			if discovered[i].Example != nil {
				def.Example = discovered[i].Example
			}
		}
		if meta, ok := discoveredByName[routeName]; ok {
			def = mergeParamDefs([]paramDef{def}, []paramDef{meta})[0]
		}
		if override, ok := overrides[routeName]; ok {
			def = mergeParamDefs([]paramDef{def}, []paramDef{override})[0]
		}
		params = append(params, def)
	}
	return params
}

func mergeParamDefs(base, extra []paramDef) []paramDef {
	index := map[string]int{}
	merged := make([]paramDef, 0, len(base)+len(extra))
	for _, p := range base {
		index[paramKey(p)] = len(merged)
		merged = append(merged, p)
	}
	for _, p := range extra {
		key := paramKey(p)
		if i, ok := index[key]; ok {
			merged[i] = mergeSingleParamDef(merged[i], p)
			continue
		}
		index[key] = len(merged)
		merged = append(merged, p)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].In != merged[j].In {
			return merged[i].In == "path"
		}
		return merged[i].Name < merged[j].Name
	})
	return merged
}

func paramKey(p paramDef) string {
	in := p.In
	if in == "" {
		in = "query"
	}
	return in + ":" + p.Name
}

func mergeSingleParamDef(base, override paramDef) paramDef {
	result := base
	if override.In != "" {
		result.In = override.In
	}
	if override.Type != "" {
		result.Type = override.Type
	}
	if override.Format != "" {
		result.Format = override.Format
	}
	if override.Description != "" {
		result.Description = override.Description
	}
	if override.Example != nil {
		result.Example = override.Example
	}
	if override.Required != nil {
		result.Required = override.Required
	}
	return result
}

func inferPathParamType(name string) string {
	paramType, _, _ := inferParamSchema(name)
	return paramType
}

func inferQueryParamType(name string) string {
	switch name {
	case "page", "per_page", "category_id", "sub_category_id", "count", "user_id", "target_user_id", "order_id", "OrderId":
		return "integer"
	case "load_buildings", "user_features_location", "recieved", "liked":
		return "boolean"
	case "points", "karbari":
		return "array"
	default:
		return "string"
	}
}

func queryTypeFromHint(hint string) string {
	switch strings.ToLower(hint) {
	case "bool", "boolean":
		return "boolean"
	case "int", "integer":
		return "integer"
	case "array":
		return "array"
	default:
		return "string"
	}
}

func isRequiredQueryParam(name string) bool {
	switch name {
	case "points", "start_date", "end_date", "state", "code":
		return true
	default:
		return false
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
