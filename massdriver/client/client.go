package client

import (
	"github.com/Khan/genqlient/graphql"
	"github.com/go-resty/resty/v2"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/config"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
)

type Client struct {
	Auth *config.Auth
	HTTP *resty.Client
	GQL  graphql.Client
}

func New() (*Client, error) {
	cfg, cfgErr := config.Get()
	if cfgErr != nil {
		return nil, cfgErr
	}

	auth, err := config.ResolveAuth(cfg)
	if err != nil {
		return nil, err
	}

	http := resty.New().
		SetBaseURL(auth.URL).
		SetHeader("Authorization", auth.Value).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json")

	gqlClient := gql.NewClient(auth)

	return &Client{
		Auth: auth,
		HTTP: http,
		GQL:  gqlClient,
	}, nil
}
