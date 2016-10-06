package backplane

import (
	"os"
	"testing"
)

func TestAuthenticate(t *testing.T) {
	token := os.Getenv("BACKPLANE_TOKEN")
	c, err := New(token)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}

	err = c.API("GET", "/q", map[string]string{}, nil, &result)
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuery(t *testing.T) {
	token := os.Getenv("BACKPLANE_TOKEN")
	c, err := New(token)
	if err != nil {
		t.Fatal(err)
	}

	result := &QueryResponse{}

	err = c.API("GET", "/q", nil, nil, result)
	if err != nil {
		t.Fatal(err)
	}
}
