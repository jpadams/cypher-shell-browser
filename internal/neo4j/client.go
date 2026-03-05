package neo4j

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Client struct {
	driver   neo4j.DriverWithContext
	database string
}

func New(uri, username, password, database string) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create driver: %w", err)
	}
	return &Client{driver: driver, database: database}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.driver.VerifyConnectivity(ctx)
}

func (c *Client) Run(ctx context.Context, cypher string, params map[string]any) (*QueryResult, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.database})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return collectResult(ctx, result)
}

func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}
