package backplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const (
	backplaneURL  = "https://www.backplane.io"
	backplaneHost = "www.backplane.io"
)

type Client struct {
	token string
}

func New(token string) (*Client, error) {
	return &Client{token}, nil
}

func (c *Client) setBasicAuth(req *http.Request) error {
	req.SetBasicAuth(c.token, "")

	return nil
}

func (c *Client) API(method, path string, parameters map[string]string, postData interface{}, out interface{}) error {
	v := url.Values{}

	req, err := http.NewRequest(method, backplaneURL+path, nil)
	if err != nil {
		return err
	}

	if method == "POST" || method == "PUT" {
		out := &bytes.Buffer{}
		err = json.NewEncoder(out).Encode(postData)
		if err != nil {
			return err
		}

		req.Body = ioutil.NopCloser(out)
	}

	for key, value := range parameters {
		v.Add(key, value)
	}

	req.URL.RawQuery = v.Encode()
	c.setBasicAuth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		rbody, _ := ioutil.ReadAll(resp.Body)
		return errors.New("backplane: request failed " + string(rbody))
	}

	if out != nil {
		err = json.NewDecoder(resp.Body).Decode(out)
		if err != nil {
			return err
		}
	}

	return nil
}

type Route struct {
	ID          string   `json:"ID,omitempty"`
	RawSelector string   `json:"RawSelector"`
	Weight      int      `json:"Weight,omitempty"`
	Strategy    string   `json:"Strategy,omitempty"`
	Backends    []string `json:"Backends,omitempty"`
}

type Location struct {
	Latitude      float64 `json:"Latitude"`
	Longitude     float64 `json:"Longitude"`
	CityName      string  `json:"CityName"`
	CountryCode   string  `json:"CountryCode"`
	CountryName   string  `json:"CountryName"`
	ContinentCode string  `json:"ContinentCode"`
	ContinentName string  `json:"ContinentName"`
	RegionCode    string  `json:"RegionCode"`
	RegionName    string  `json:"RegionName"`
}

type Backend struct {
	ID                string    `json:"ID"`
	Owner             string    `json:"Owner"`
	RawLabels         string    `json:"RawLabels"`
	Load              int       `json:"Load"`
	RemoteAddr        string    `json:"RemoteAddr"`
	ConnectedAt       time.Time `json:"ConnectedAt"`
	Location          Location  `json:"Location"`
	RequestsPerSecond int       `json:"RequestsPerSecond"`
	State             string    `json:"State"`
}

type Endpoint struct {
	Pattern   string    `json:"Pattern"`
	Owner     string    `json:"Owner"`
	CreatedAt time.Time `json:"CreatedAt"`
	Routes    []Route   `json:"Routes"`
}

type QueryResponse struct {
	Token     string     `json:"Token"`
	Endpoints []Endpoint `json:"Endpoints"`
	Backends  []Backend  `json:"Backends"`
}

func (c *Client) Query() (*QueryResponse, error) {
	result := &QueryResponse{}
	err := c.API("GET", "/q", nil, nil, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type RouteRequest struct {
	Pattern string `json:"Pattern"`
	Route   Route
}

func (c *Client) Route(pattern string, labels map[string]string) (*Route, error) {
	var flatSelectors string

	for key, value := range labels {
		flatSelectors = flatSelectors + ", " + key + "=" + value
	}

	req := &RouteRequest{
		Pattern: pattern,
		Route: Route{
			RawSelector: flatSelectors,
		},
	}
	result := &Route{}

	err := c.API("POST", "/route", nil, req, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type ShapeRequest struct {
	Pattern string
	Routes  []Route
}

func (c *Client) Shape(endpoint string, weights map[string]int) error {
	routes := []Route{}

	for route, weight := range weights {
		routes = append(routes, Route{
			ID:     route,
			Weight: weight,
		})
	}

	req := &ShapeRequest{
		Pattern: endpoint,
		Routes:  routes,
	}

	err := c.API("POST", "/shape", nil, req, nil)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GenToken() (string, error) {
	qOutput, err := c.Query()
	if err != nil {
		return "", err
	}

	return qOutput.Token, nil
}
