package main

import "testing"

func TestResolveRequestBodyUsesSchemaAsIs(t *testing.T) {
	schema, required, description := resolveRequestBody(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"coupon_code": map[string]interface{}{"type": "string"},
		},
	})
	if !required || description != "" {
		t.Fatalf("required=%v description=%q", required, description)
	}
	asMap, ok := schema.(map[string]interface{})
	if !ok || asMap["type"] != "object" {
		t.Fatalf("schema=%#v", schema)
	}
}

func TestResolveRequestBodyWrapper(t *testing.T) {
	schema, required, description := resolveRequestBody(map[string]interface{}{
		"required":    false,
		"description": "Optional coupon",
		"schema": map[string]interface{}{
			"type": "object",
		},
	})
	if required || description != "Optional coupon" {
		t.Fatalf("required=%v description=%q", required, description)
	}
	asMap, ok := schema.(map[string]interface{})
	if !ok || asMap["type"] != "object" {
		t.Fatalf("schema=%#v", schema)
	}
}

func TestBuildSpecEmitsRouteBodyOnPostOnly(t *testing.T) {
	routes := []routeDef{{
		Path:     "/api/features/{feature}/building/enter",
		Methods:  []string{"post"},
		Tag:      "Features",
		Summary:  "Enter a building",
		Security: "bearer",
		Body: map[string]interface{}{
			"required":    false,
			"description": "Coupon is optional",
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"coupon_code": map[string]interface{}{"type": "string"},
				},
			},
		},
		Responses: map[string]map[string]interface{}{
			"200": {
				"description": "Entered",
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"message": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
	}}

	spec := buildSpec(routes, nil, "http://localhost:8000", nil)
	pathItem := spec.Paths["/api/features/{feature}/building/enter"].(map[string]interface{})
	post := pathItem["post"].(map[string]interface{})
	requestBody := post["requestBody"].(map[string]interface{})
	if requestBody["required"] != false {
		t.Fatalf("requestBody=%#v", requestBody)
	}
	if requestBody["description"] != "Coupon is optional" {
		t.Fatalf("description=%#v", requestBody["description"])
	}
	content := requestBody["content"].(map[string]interface{})
	jsonBody := content["application/json"].(map[string]interface{})
	schema := jsonBody["schema"].(map[string]interface{})
	props := schema["properties"].(map[string]interface{})
	if _, ok := props["coupon_code"]; !ok {
		t.Fatalf("schema=%#v", schema)
	}

	responses := post["responses"].(map[string]interface{})
	okResp := responses["200"].(map[string]interface{})
	if okResp["description"] != "Entered" {
		t.Fatalf("200=%#v", okResp)
	}
}
