package threed_client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client handles communication with the 3D Meta API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new 3D Meta API client
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// BuildPackageRequest represents the request parameters for build package
type BuildPackageRequest struct {
	FeatureID uint64 `json:"feature_id"`
	Area      string `json:"area"`
	Density   string `json:"density"`
	Karbari   string `json:"karbari"`
	Page      int32  `json:"page"`
}

// BuildPackageResponse represents the response from the build package API
type BuildPackageResponse struct {
	Data []BuildingModelData `json:"data"`
}

// BuildingModelData represents a building model from the 3D API
type BuildingModelData struct {
	ID         uint64                   `json:"id"`
	Name       string                   `json:"name"`
	SKU        string                   `json:"sku"`
	Images     []map[string]interface{} `json:"images"`
	Attributes []map[string]interface{} `json:"attributes"`
	File       map[string]interface{}   `json:"file"`
}

// GetBuildPackage calls the 3D Meta API to get available building models
func (c *Client) GetBuildPackage(req BuildPackageRequest) (*BuildPackageResponse, error) {
	params := url.Values{}
	params.Add("feature_id", fmt.Sprintf("%d", req.FeatureID))
	params.Add("area", req.Area)
	params.Add("density", req.Density)
	params.Add("karbari", req.Karbari)
	params.Add("page", fmt.Sprintf("%d", req.Page))

	apiURL := fmt.Sprintf("%s/api/v1/build-package?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call 3D Meta API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("3D Meta API returned error: %s - %s", resp.Status, string(body))
	}

	var result BuildPackageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode 3D Meta API response: %w", err)
	}

	return &result, nil
}
