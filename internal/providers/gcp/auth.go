package gcp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"cloud.google.com/go/bigquery"
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	recommender "cloud.google.com/go/recommender/apiv1"
	"golang.org/x/oauth2"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1beta4"
)

var gcpExecCommandContext = exec.CommandContext

func newComputeService(ctx context.Context) (*compute.Service, error) {
	return gcpServiceWithCLIAuthFallback(ctx, compute.NewService)
}

func newContainerService(ctx context.Context) (*container.Service, error) {
	return gcpServiceWithCLIAuthFallback(ctx, container.NewService)
}

func newSQLAdminService(ctx context.Context) (*sqladmin.Service, error) {
	return gcpServiceWithCLIAuthFallback(ctx, sqladmin.NewService)
}

func newRecommenderClient(ctx context.Context) (*recommender.Client, error) {
	return gcpServiceWithCLIAuthFallback(ctx, recommender.NewClient)
}

func newMonitoringMetricClient(ctx context.Context) (*monitoring.MetricClient, error) {
	return gcpServiceWithCLIAuthFallback(ctx, monitoring.NewMetricClient)
}

func newBigQueryClient(ctx context.Context, projectID string) (*bigquery.Client, error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err == nil {
		return client, nil
	}

	opts, fallbackErr := gcpCLIAuthOptions(ctx)
	if fallbackErr != nil {
		return nil, gcpWrapAuthError(err, fallbackErr)
	}

	client, fallbackErr = bigquery.NewClient(ctx, projectID, opts...)
	if fallbackErr != nil {
		return nil, gcpWrapAuthError(err, fallbackErr)
	}
	return client, nil
}

func gcpServiceWithCLIAuthFallback[T any](ctx context.Context, create func(context.Context, ...option.ClientOption) (*T, error)) (*T, error) {
	svc, err := create(ctx)
	if err == nil {
		return svc, nil
	}

	opts, fallbackErr := gcpCLIAuthOptions(ctx)
	if fallbackErr != nil {
		return nil, gcpWrapAuthError(err, fallbackErr)
	}

	svc, fallbackErr = create(ctx, opts...)
	if fallbackErr != nil {
		return nil, gcpWrapAuthError(err, fallbackErr)
	}
	return svc, nil
}

func gcpCLIAuthOptions(ctx context.Context) ([]option.ClientOption, error) {
	token, err := gcpAccessTokenFromCLI(ctx)
	if err != nil {
		return nil, err
	}
	return []option.ClientOption{
		option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})),
	}, nil
}

func gcpAccessTokenFromCLI(ctx context.Context) (string, error) {
	cmd := gcpExecCommandContext(ctx, "gcloud", "auth", "print-access-token")
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return "", fmt.Errorf("gcloud auth print-access-token failed: %v: %s", err, message)
		}
		return "", fmt.Errorf("gcloud auth print-access-token failed: %w", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("gcloud auth print-access-token returned an empty token")
	}
	return token, nil
}

func gcpWrapAuthError(primary, fallback error) error {
	return fmt.Errorf("%w. CloudManager also tried the active gcloud login, but that failed: %v. Fix with 'gcloud auth application-default login', set GOOGLE_APPLICATION_CREDENTIALS, or ensure 'gcloud auth login' works in this shell", primary, fallback)
}
