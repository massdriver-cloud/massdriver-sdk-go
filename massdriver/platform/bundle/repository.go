package bundle

import (
	"net/http"
	"net/url"
	"path"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/client"

	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func GetBundleRepository(mdClient *client.Client, bundleName string) (oras.Target, error) {
	mdUrl, urlErr := url.Parse(mdClient.Config.URL)
	if urlErr != nil {
		return nil, urlErr
	}
	mdHost := mdUrl.Host

	repo, repoErr := remote.NewRepository(path.Join(mdHost, mdClient.Config.OrganizationID, bundleName))
	if repoErr != nil {
		return nil, repoErr
	}

	if mdUrl.Scheme == "http" {
		repo.PlainHTTP = true
	}

	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Header: http.Header{
			"authorization": []string{mdClient.Config.Credentials.AuthHeaderValue},
		},
	}

	return repo, nil
}
