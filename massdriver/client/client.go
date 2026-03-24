package client

import (
	"github.com/Khan/genqlient/graphql"
	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
)

type Client struct {
	Config config.Config
	HTTP   *resty.Client
	GQL    graphql.Client
	GQLv1  graphql.Client
}

func New() (*Client, error) {
	cfg, cfgErr := config.Get()
	if cfgErr != nil {
		return nil, cfgErr
	}

	http := resty.New().
		SetBaseURL(cfg.URL).
		SetHeader("Authorization", cfg.Credentials.AuthHeaderValue).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	return &Client{
		Config: *cfg,
		HTTP:   http,
		GQL:    gql.NewV0Client(cfg),
		GQLv1:  gql.NewV1Client(cfg),
	}, nil
}
