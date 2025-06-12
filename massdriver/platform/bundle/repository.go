package bundle

import (
	"net/url"
	"path/filepath"

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

	reg := mdUrl.Host
	repo, repoErr := remote.NewRepository(filepath.Join(reg, mdClient.Config.OrganizationID, bundleName))
	if repoErr != nil {
		return nil, repoErr
	}

	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(reg, auth.Credential{
			Username: mdClient.Config.Credentials.ID,
			Password: mdClient.Config.Credentials.Secret,
		}),
	}

	return repo, nil
}
