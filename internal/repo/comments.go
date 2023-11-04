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

var graphLookupStep = bson.D{
	{
		Key: "$graphLookup",
		Value: bson.M{
			"from":             CommentCollection,
			"startWith":        "$_id",
			"connectFromField": "_id",
			"connectToField":   "parentId",
			"as":               "commentTree",
		},
	},
}

func (r *Repository) CreateComment(ctx context.Context, model models.Comment) (string, error) {
	// verify that the scope actually exists
	if _, err := r.GetScopeByID(ctx, model.Scope); err != nil {
		return "", err
	}

	if model.ID.IsZero() {
		model.ID = primitive.NewObjectID()
	}

	// insert the actual scope
	if _, err := r.comments.InsertOne(ctx, model); err != nil {
		return "", err
	}

	return model.ID.Hex(), nil
}

func (r *Repository) GetComment(ctx context.Context, id string) (models.Comment, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.Comment{}, connect.NewError(connect.CodeInvalidArgument, err)
	}

	res := r.comments.FindOne(ctx, bson.M{"_id": oid})
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return models.Comment{}, connect.NewError(connect.CodeNotFound, fmt.Errorf("comment not found"))
		}

		return models.Comment{}, err
	}

	var c models.Comment
	if err := res.Decode(&c); err != nil {
		return models.Comment{}, fmt.Errorf("failed to decode comment: %w", err)
	}

	return c, nil
}

func (r *Repository) GetParentComments(ctx context.Context, id string) ([]models.Comment, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	pipeline := mongo.Pipeline{
		{{
			Key: "$match",
			Value: bson.M{
				"_id": oid,
			},
		}},
		{{
			Key: "$graphLookup",
			Value: bson.M{
				"from":             CommentCollection,
				"startWith":        "$_id",
				"connectFromField": "parentId",
				"connectToField":   "_id",
				"as":               "commentTree",
			},
		}},
	}

	res, err := r.comments.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	var result []treeResult
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("comment not found"))
	}

	return result[0].Tree, nil
}

func (r *Repository) GetCommentTreeFromCommentID(ctx context.Context, id string) (*models.CommentTree, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	pipeline := mongo.Pipeline{
		{{
			Key: "$match",
			Value: bson.M{
				"_id": oid,
			},
		}},
		graphLookupStep,
	}

	res, err := r.comments.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	var result []treeResult
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("comment not found"))
	}

	return result[0].buildCommentTree()
}

func (r *Repository) GetCommentTreeByScope(ctx context.Context, scopeId string, reference string) ([]*models.CommentTree, error) {
	filter := bson.M{
		"scopeId": scopeId,
		"parentId": bson.M{
			"$exists": false,
		},
	}

	if reference != "" {
		filter["ref"] = reference
	}

	pipeline := mongo.Pipeline{
		{{
			Key:   "$match",
			Value: filter,
		}},
		graphLookupStep,
	}

	res, err := r.comments.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	var result []treeResult
	if err := res.All(ctx, &result); err != nil {
		return nil, err
	}

	trees := make([]*models.CommentTree, len(result))
	for idx, r := range result {
		trees[idx], err = r.buildCommentTree()
		if err != nil {
			return nil, fmt.Errorf("failed to build comment tree for %q: %w", r.Comment.ID.Hex(), err)
		}
	}

	return trees, nil
}

type treeResult struct {
	models.Comment `bson:",inline"`
	Tree           []models.Comment `bson:"commentTree"`
}

func (tr treeResult) buildCommentTree() (*models.CommentTree, error) {
	// build up a valid comment tree
	trees := make(map[string]*models.CommentTree)

	resultTree := &models.CommentTree{
		Comment: tr.Comment,
	}

	// first, create a tree object for each comment
	for _, c := range tr.Tree {
		// append a new tree object for each answer command
		tree, ok := trees[c.ID.Hex()]
		if !ok {
			tree = &models.CommentTree{
				Comment: c,
			}
		}
		trees[c.ID.Hex()] = tree
	}

	// next, fill up the answers for each tree object
	for _, c := range tr.Tree {
		tree := trees[c.ID.Hex()]

		switch {
		case c.ParentID.Hex() == tr.Comment.ID.Hex():
			resultTree.Answers = append(resultTree.Answers, tree)

		default:
			parentTree, ok := trees[c.ParentID.Hex()]
			if !ok {
				return nil, fmt.Errorf("failed to get parent tree for id %q (parent: %q)", c.ID.Hex(), c.ParentID.Hex())
			}

			parentTree.Answers = append(parentTree.Answers, tree)
		}
	}

	return resultTree, nil
}
