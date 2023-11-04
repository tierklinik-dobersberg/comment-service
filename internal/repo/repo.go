package repo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

const (
	ScopeCollection   = "scopes"
	CommentCollection = "comments"
)

type Repository struct {
	cli      *mongo.Client
	db       string
	scopes   *mongo.Collection
	comments *mongo.Collection
}

func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	// parse the connection string and make sure we have a database specified.
	connStr, err := connstring.ParseAndValidate(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection string: %w", err)
	}

	if connStr.Database == "" {
		connStr.Database = "comment-service:v1"
	}

	// create a mongo-db client
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	db := cli.Database(connStr.Database)

	// try to ping mongo
	if err := cli.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	r := &Repository{
		cli:      cli,
		db:       connStr.Database,
		scopes:   db.Collection(ScopeCollection),
		comments: db.Collection(CommentCollection),
	}

	if err := r.prepare(ctx); err != nil {
		return r, fmt.Errorf("failed to prepare collections: %w", err)
	}

	return r, nil
}

func (repo *Repository) prepare(ctx context.Context) error {
	_, err := repo.comments.Indexes().
		CreateMany(ctx, []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: "creator_id", Value: 1},
				},
			},
		})

	if err != nil {
		return fmt.Errorf("failed to create comment indexes: %w", err)
	}

	_, err = repo.scopes.Indexes().
		CreateMany(ctx, []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: "name", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{
					{Key: "scopeId", Value: 1},
				},
				Options: options.Index().SetUnique(true),
			},
		})

	if err != nil {
		return fmt.Errorf("failed to create scope indexes: %w", err)
	}

	return nil
}
