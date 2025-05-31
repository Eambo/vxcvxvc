package database

import (
	"github.com/elastic/go-elasticsearch/v8"
)

// GetESClient initializes and returns an Elasticsearch client.
// It connects to a default local Elasticsearch instance.
func GetESClient() (*elasticsearch.Client, error) {
	es, err := elasticsearch.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	return es, nil
}
