package trafficmanager

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/trafficmanager/armtrafficmanager"
	"go.uber.org/zap"
)

type testCredential struct{}

func (testCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) Do(r *http.Request) (*http.Response, error) { return f(r) }

func TestSDKDeletionRetries(t *testing.T) {
	for _, resource := range []string{"endpoint", "profile"} {
		for _, tc := range []struct {
			name      string
			statuses  []int
			wantError bool
		}{
			{"success", []int{200}, false},
			{"throttled", []int{429, 200}, false},
			{"transient", []int{503, 200}, false},
			{"permanent", []int{403}, true},
			{"already-deleted", []int{404}, false},
			{"exhausted", []int{503, 503, 503}, true},
		} {
			t.Run(resource+"/"+tc.name, func(t *testing.T) {
				attempts := 0
				options := &arm.ClientOptions{ClientOptions: azcore.ClientOptions{
					Retry: policy.RetryOptions{MaxRetries: 2, RetryDelay: time.Millisecond, MaxRetryDelay: time.Millisecond},
					Transport: testTransport(func(request *http.Request) (*http.Response, error) {
						if request.Method != http.MethodDelete {
							t.Fatalf("unexpected method %s", request.Method)
						}
						if attempts >= len(tc.statuses) {
							t.Fatal("too many attempts")
						}
						status := tc.statuses[attempts]
						attempts++
						return &http.Response{StatusCode: status, Request: request,
							Header: http.Header{"Content-Type": []string{"application/json"}},
							Body:   io.NopCloser(strings.NewReader(`{"error":{"code":"TestError","message":"injected"}}`)),
						}, nil
					}),
				}}
				profiles, err := armtrafficmanager.NewProfilesClient("test-subscription", testCredential{}, options)
				if err != nil {
					t.Fatal(err)
				}
				endpoints, err := armtrafficmanager.NewEndpointsClient("test-subscription", testCredential{}, options)
				if err != nil {
					t.Fatal(err)
				}
				client := &Client{profilesClient: profiles, endpointsClient: endpoints, logger: zap.NewNop()}
				if resource == "profile" {
					err = client.DeleteProfile(context.Background(), "rg", "profile")
				} else {
					err = client.DeleteEndpoint(context.Background(), "rg", "profile", "ExternalEndpoints", "east")
				}
				if (err != nil) != tc.wantError || attempts != len(tc.statuses) {
					t.Fatalf("error=%v, attempts=%d; want error=%v, attempts=%d", err, attempts, tc.wantError, len(tc.statuses))
				}
				if tc.wantError {
					var responseError *azcore.ResponseError
					if !errors.As(err, &responseError) {
						t.Fatalf("SDK error classification lost: %v", err)
					}
				}
			})
		}
	}
}

func TestIsManagedByUs(t *testing.T) {
	for _, tc := range []struct {
		tags map[string]*string
		want bool
	}{
		{nil, false},
		{map[string]*string{}, false},
		{map[string]*string{"managedBy": nil}, false},
		{map[string]*string{"managedBy": toStringPtr("another-controller")}, false},
		{map[string]*string{"managedBy": toStringPtr("external-dns-traffic-manager-webhook")}, true},
	} {
		if got := isManagedByUs(&armtrafficmanager.Profile{Tags: tc.tags}); got != tc.want {
			t.Errorf("tags=%v: got %v, want %v", tc.tags, got, tc.want)
		}

	}
}

func TestIsNotFound(t *testing.T) {
	if IsNotFound(errors.New("transport failed")) || IsNotFound(nil) {
		t.Fatal("only an Azure 404 is a missing resource")
	}
	if !IsNotFound(&azcore.ResponseError{StatusCode: http.StatusNotFound}) {
		t.Fatal("Azure 404 not recognized")
	}
}
