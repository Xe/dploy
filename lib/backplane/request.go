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

// Client is the API client that performs actions against the Backplane API server.
type Client struct {
	token string
}

// New creates a new API client with the given token
func New(token string) (*Client, error) {
	return &Client{token}, nil
}

// setBasicAuth adds needed authentication information to a HTTP request.
func (c *Client) setBasicAuth(req *http.Request) error {
	req.SetBasicAuth(c.token, "")

	return nil
}

// API is a generic json-encoding like function that allows access to any backplane.io API call.
//
// See the implementation of Client.Query and Client.Shape for usage information.
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

// Route is how Backplane determines how to get traffic from an Endpoint to a Backend. Routes are identified by their routeID, which looks like route000. Routes are chosen by their Endpoint based on their weight.
//
// Once a Route has been chosen Backplane will then choose a Backend.
type Route struct {
	ID          string   `json:"ID,omitempty"`
	RawSelector string   `json:"RawSelector"`
	Weight      int      `json:"Weight,omitempty"`
	Strategy    string   `json:"Strategy,omitempty"`
	Backends    []string `json:"Backends,omitempty"`
}

// Location represents a geographic location.
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

// Backend is a HTTP web server connected to Backplane via the backplane agent paired to it.
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

// Endpoint looks like example.com api.example.com, or www.example.com/blog and are matched to requests with URLs that most closely match them.
type Endpoint struct {
	Pattern   string    `json:"Pattern"`
	Owner     string    `json:"Owner"`
	CreatedAt time.Time `json:"CreatedAt"`
	Routes    []Route   `json:"Routes"`
}

// QueryResponse matches the output of www.backplane.io/q
type QueryResponse struct {
	Token     string     `json:"Token"`
	Endpoints []Endpoint `json:"Endpoints"`
	Backends  []Backend  `json:"Backends"`
}

// Query fetches infornation about all Endpoints, Routes and Backends registered to your account.
func (c *Client) Query() (*QueryResponse, error) {
	result := &QueryResponse{}
	err := c.API("GET", "/q", nil, nil, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type routeRequest struct {
	Pattern string `json:"Pattern"`
	Route   Route
}

// Route creates a new Route on Backplane for the given pattern and label selector.
func (c *Client) Route(pattern string, labels map[string]string) (*Route, error) {
	var flatSelectors string

	for key, value := range labels {
		flatSelectors = flatSelectors + ", " + key + "=" + value
	}

	req := &routeRequest{
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

type shapeRequest struct {
	Pattern string
	Routes  []Route
}

// Shape changes the weights on Routes of a given Endpoint.
func (c *Client) Shape(endpoint string, weights map[string]int) error {
	routes := []Route{}

	for route, weight := range weights {
		routes = append(routes, Route{
			ID:     route,
			Weight: weight,
		})
	}

	req := &shapeRequest{
		Pattern: endpoint,
		Routes:  routes,
	}

	err := c.API("POST", "/shape", nil, req, nil)
	if err != nil {
		return err
	}

	return nil
}

// GenToken creates a new Backplane API token for a future backplane Agent to user.
func (c *Client) GenToken() (string, error) {
	qOutput, err := c.Query()
	if err != nil {
		return "", err
	}

	return qOutput.Token, nil
}
