package listcmd_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cli/cli/v2/api"
	listcmd "github.com/cli/cli/v2/pkg/cmd/sponsors/list"
	"github.com/stretchr/testify/require"
)

type gqlQuery struct {
	Query     string
	Variables map[string]any
}

func TestGQLSponsorListing(t *testing.T) {
	// TODO: consider extracting some shared server abstraction here to clean up the test, if it's useful.

	// Given the server returns a valid query response
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First we ensure the body can be successfully decoded as a GQL query
		var q gqlQuery
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		// Then we check that the query is as expected
		expectedQuery := "query ListSponsors($login:String!){user(login: $login){sponsors(first: 30){nodes{... on Organization{login},... on User{login}}}}}"
		if q.Query != expectedQuery {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf("expected query: '%s' but got '%s'", expectedQuery, q.Query)))
			return
		}

		expectedUsername := "testusername"
		if q.Variables["login"] != expectedUsername {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf("expected username: '%s' but got '%s'", expectedUsername, q.Variables["login"])))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"user": {
					"sponsors": {
						"nodes": [
							{
								"login": "sponsor1"
							},
							{
								"login": "sponsor2"
							}
						]
					}
				}
			}
		}`))
	}))
	// Unfortunately, because the APIClient does some nonsense for to prefix URLs with https we have to start
	// the server with TLS and then below we InsecureSkipVerify
	s.StartTLS()
	t.Cleanup(s.Close)

	c := listcmd.GQLSponsorClient{
		Hostname: s.Listener.Addr().String(),
		APIClient: api.NewClientFromHTTP(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}),
	}

	// When we list the sponsors
	listedSponsors, err := c.ListSponsors("testusername")

	// Then we expect no error
	require.NoError(t, err)

	// And the sponsors match the query response
	require.Equal(t, []listcmd.Sponsor{"sponsor1", "sponsor2"}, listedSponsors)
}

func TestGQLSponsorListingServerError(t *testing.T) {
	// Given the server is returning an error
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	// Unfortunately, because the APIClient does some nonsense for to prefix URLs with https we have to start
	// the server with TLS and then below we InsecureSkipVerify
	s.StartTLS()
	t.Cleanup(s.Close)

	c := listcmd.GQLSponsorClient{
		Hostname: s.Listener.Addr().String(),
		APIClient: api.NewClientFromHTTP(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}),
	}

	// When we list the sponsors
	_, err := c.ListSponsors("testusername")

	// Then we expect to see a useful error
	require.ErrorContains(t, err, "list sponsors: non-200 OK status code")
}
