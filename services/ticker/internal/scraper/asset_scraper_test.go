package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	hProtocol "github.com/shantanu-hashcash/go/protocols/aurora"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/support/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldDiscardAsset(t *testing.T) {
	testAsset := hProtocol.AssetStat{
		Amount: "",
	}

	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount: "0.0",
	}
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount: "0",
	}
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 8,
	}
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 12,
	}
	testAsset.Code = "REMOVE"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 100,
	}
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = ""
	assert.Equal(t, shouldDiscardAsset(testAsset, true), false)

	testAsset = hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 40,
	}
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = "http://www.hcnet.org/.well-known/hcnet.toml"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 40,
	}
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = ""
	assert.Equal(t, shouldDiscardAsset(testAsset, true), true)

	testAsset = hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 40,
	}
	testAsset.Code = "SOMETHINGVALID"
	testAsset.Links.Toml.Href = "https://www.hcnet.org/.well-known/hcnet.toml"
	assert.Equal(t, shouldDiscardAsset(testAsset, true), false)
}

func TestDomainsMatch(t *testing.T) {
	tomlURL, _ := url.Parse("https://hcnet.org/hcnet.toml")
	orgURL, _ := url.Parse("https://hcnet.org/")
	assert.True(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://assets.hcnet.org/hcnet.toml")
	orgURL, _ = url.Parse("https://hcnet.org/")
	assert.False(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://hcnet.org/hcnet.toml")
	orgURL, _ = url.Parse("https://home.hcnet.org/")
	assert.True(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://hcnet.org/hcnet.toml")
	orgURL, _ = url.Parse("https://home.hcnet.com/")
	assert.False(t, domainsMatch(tomlURL, orgURL))

	tomlURL, _ = url.Parse("https://hcnet.org/hcnet.toml")
	orgURL, _ = url.Parse("https://hcnet.com/")
	assert.False(t, domainsMatch(tomlURL, orgURL))
}

func TestIsDomainVerified(t *testing.T) {
	tomlURL := "https://hcnet.org/hcnet.toml"
	orgURL := "https://hcnet.org/"
	hasCurrency := true
	assert.True(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://hcnet.org/hcnet.toml"
	orgURL = ""
	hasCurrency = true
	assert.True(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = ""
	orgURL = ""
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://hcnet.org/hcnet.toml"
	orgURL = "https://hcnet.org/"
	hasCurrency = false
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "http://hcnet.org/hcnet.toml"
	orgURL = "https://hcnet.org/"
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://hcnet.org/hcnet.toml"
	orgURL = "http://hcnet.org/"
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))

	tomlURL = "https://hcnet.org/hcnet.toml"
	orgURL = "https://hcnet.com/"
	hasCurrency = true
	assert.False(t, isDomainVerified(orgURL, tomlURL, hasCurrency))
}

func TestIgnoreInvalidTOMLUrls(t *testing.T) {
	invalidURL := "https:// there is something wrong here.com/hcnet.toml"
	_, err := fetchTOMLData(invalidURL)

	urlErr, ok := errors.Cause(err).(*url.Error)
	if !ok {
		t.Fatalf("err expected to be a url.Error but was %#v", err)
	}
	assert.Equal(t, "parse", urlErr.Op)
	assert.Equal(t, "https:// there is something wrong here.com/hcnet.toml", urlErr.URL)
	assert.EqualError(t, urlErr.Err, `invalid character " " in host name`)
}

func TestProcessAsset_notCached(t *testing.T) {
	logger := log.DefaultLogger
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `SIGNING_KEY="not cached signing key"`)
	}))
	asset := hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 100,
	}
	asset.Code = "SOMETHINGVALID"
	asset.Links.Toml.Href = server.URL
	tomlCache := &TOMLCache{}
	finalAsset, err := processAsset(logger, asset, tomlCache, true)
	require.NoError(t, err)
	assert.NotZero(t, finalAsset)
	assert.Equal(t, "not cached signing key", finalAsset.IssuerDetails.SigningKey)
	cachedTOML, ok := tomlCache.Get(server.URL)
	assert.True(t, ok)
	assert.Equal(t, TOMLIssuer{SigningKey: "not cached signing key"}, cachedTOML)
}

func TestProcessAsset_cached(t *testing.T) {
	logger := log.DefaultLogger
	asset := hProtocol.AssetStat{
		Amount:      "123901.0129310",
		NumAccounts: 100,
	}
	asset.Code = "SOMETHINGVALID"
	asset.Links.Toml.Href = "url"
	tomlCache := &TOMLCache{}
	tomlCache.Set("url", TOMLIssuer{SigningKey: "signing key"})
	finalAsset, err := processAsset(logger, asset, tomlCache, true)
	require.NoError(t, err)
	assert.NotZero(t, finalAsset)
	assert.Equal(t, "signing key", finalAsset.IssuerDetails.SigningKey)
}
