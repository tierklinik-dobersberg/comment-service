package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bufbuild/connect-go"
	"github.com/tierklinik-dobersberg/comment-service/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (r *Repository) CreateScope(ctx context.Context, model *models.Scope) (id string, err error) {
	if model.InternalID.IsZero() {
		model.InternalID = primitive.NewObjectID()
	}

	if _, err := r.scopes.InsertOne(ctx, model); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("scope id already exists"))
		}

		return "", fmt.Errorf("failed to save scope: %w", err)
	}

	return model.InternalID.Hex(), nil
}

func (r *Repository) UpdateScope(ctx context.Context, id string, model *models.Scope) error {
	res, err := r.scopes.ReplaceOne(ctx, bson.M{"scopeId": id}, model)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("scope id not found"))
	}

	return nil
}

func (r *Repository) GetScopeByID(ctx context.Context, id string) (models.Scope, error) {
	res := r.scopes.FindOne(ctx, bson.M{"scopeId": id})
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Scope{}, connect.NewError(connect.CodeNotFound, fmt.Errorf("failed to find scope"))
		}

		return models.Scope{}, fmt.Errorf("failed")
	}

	var scope models.Scope
	if err := res.Decode(&scope); err != nil {
		return models.Scope{}, fmt.Errorf("failed to decode scope: %w", err)
	}

	return scope, nil
}

func (r *Repository) DeleteScope(ctx context.Context, id string, recurseComment bool) error {
	res, err := r.scopes.DeleteOne(ctx, bson.M{"scopeId": id})
	if err != nil {
		return fmt.Errorf("failed to delete scope: %w", err)
	}

	if res.DeletedCount == 0 {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("scope not found"))
	}

	if recurseComment {
		_, err := r.comments.DeleteMany(ctx, bson.M{
			"scopeId": id,
		})
		if err != nil {
			return fmt.Errorf("failed to delete comments: %w", err)
		}
	}

	return nil
}

func (r *Repository) ListScopes(ctx context.Context) ([]models.Scope, error) {
	res, err := r.scopes.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to find scopes: %w", err)
	}

	var result []models.Scope
	if err := res.All(ctx, &result); err != nil {
		return result, fmt.Errorf("failed to decode scopes: %w", err)
	}

	return result, nil
}
