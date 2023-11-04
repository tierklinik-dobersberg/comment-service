package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/hashicorp/go-multierror"
	"github.com/mennanov/fmutils"
	commentv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1/commentv1connect"
	idmv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/data"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/comment-service/internal/config"
	"github.com/tierklinik-dobersberg/comment-service/internal/goldmark-extensions/mentions"
	"github.com/tierklinik-dobersberg/comment-service/internal/models"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	*config.Providers

	commentv1connect.UnimplementedCommentServiceHandler
}

func New(p *config.Providers) *Service {
	return &Service{
		Providers: p,
	}
}

// Scope Management

func (svc *Service) CreateScope(ctx context.Context, req *connect.Request[commentv1.CreateScopeRequest]) (*connect.Response[commentv1.CreateScopeResponse], error) {
	scopeModel := &models.Scope{
		ID:                     req.Msg.Id,
		Name:                   req.Msg.Name,
		NotificationType:       models.NotifcationTypeFromProto(req.Msg.NotifcationType),
		CommentViewURLTemplate: req.Msg.ViewCommentUrlTemplate,
	}

	_, err := svc.Repository.CreateScope(ctx, scopeModel)
	if err != nil {
		return nil, err
	}

	scope, err := svc.Repository.GetScopeByID(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&commentv1.CreateScopeResponse{
		Scope: scope.ToProto(),
	}), nil
}

func (svc *Service) ListScope(ctx context.Context, req *connect.Request[commentv1.ListScopeRequest]) (*connect.Response[commentv1.ListScopeResponse], error) {
	scopes, err := svc.Repository.ListScopes(ctx)
	if err != nil {
		return nil, err
	}

	res := &commentv1.ListScopeResponse{
		Scopes: make([]*commentv1.Scope, len(scopes)),
	}

	for idx, s := range scopes {
		res.Scopes[idx] = s.ToProto()
	}

	return connect.NewResponse(res), nil
}

func (svc *Service) DeleteScope(ctx context.Context, req *connect.Request[commentv1.DeleteScopeRequest]) (*connect.Response[commentv1.DeleteScopeResponse], error) {
	err := svc.Repository.DeleteScope(ctx, req.Msg.Id, true)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&commentv1.DeleteScopeResponse{}), nil
}

// Comment Management

func (svc *Service) CreateComment(ctx context.Context, req *connect.Request[commentv1.CreateCommentRequest]) (*connect.Response[commentv1.CreateCommentResponse], error) {
	usr := auth.From(ctx)
	if usr == nil {
		return nil, fmt.Errorf("no remote user specified")
	}

	m := models.Comment{
		Content:   req.Msg.Content,
		CreatedAt: time.Now(),
		CreatorID: usr.ID,
	}

	switch v := req.Msg.Kind.(type) {
	case *commentv1.CreateCommentRequest_Root:
		m.Scope = v.Root.Scope
		m.Reference = v.Root.Reference

	case *commentv1.CreateCommentRequest_ParentId:
		parentComment, err := svc.Repository.GetComment(ctx, v.ParentId)
		if err != nil {
			return nil, err
		}

		m.ParentID = parentComment.ID
		m.Scope = parentComment.Scope
		m.Reference = parentComment.Reference
	}

	insertId, err := svc.Repository.CreateComment(ctx, m)
	if err != nil {
		return nil, err
	}

	// cannot fail because the ID has just been created
	m.ID, _ = primitive.ObjectIDFromHex(insertId)

	// gather parent comment creators and @-user-mentions in the comment content
	// and send appropriate notifications.
	go svc.sendNotifications(m)

	return connect.NewResponse(&commentv1.CreateCommentResponse{
		Comment: m.ToProto(),
	}), nil
}

func (svc *Service) GetComment(ctx context.Context, req *connect.Request[commentv1.GetCommentRequest]) (*connect.Response[commentv1.GetCommentResponse], error) {
	if req.Msg.Recurse {
		c, err := svc.Repository.GetCommentTreeFromCommentID(ctx, req.Msg.Id)
		if err != nil {
			return nil, err
		}

		treepb := c.ToProto(req.Msg.Recurse)

		minifyTree(treepb, true)

		return connect.NewResponse(&commentv1.GetCommentResponse{
			Result: treepb,
		}), nil
	}

	c, err := svc.Repository.GetComment(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}

	if req.Msg.RenderHtml {
		if err := svc.renderCommentInline(ctx, &c); err != nil {
			return nil, err
		}
	}

	return connect.NewResponse(&commentv1.GetCommentResponse{
		Result: &commentv1.CommentTree{
			Comment: c.ToProto(),
		},
	}), nil
}

func (svc *Service) ListComments(ctx context.Context, req *connect.Request[commentv1.ListCommentsRequest]) (*connect.Response[commentv1.ListCommentsResponse], error) {
	trees, err := svc.Repository.GetCommentTreeByScope(ctx, req.Msg.Scope, req.Msg.Reference)
	if err != nil {
		return nil, err
	}

	if len(trees) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no comments found"))
	}

	if req.Msg.RenderHtml {
		for _, t := range trees {
			if err := svc.renderCommentTree(ctx, t); err != nil {
				return nil, err
			}
		}
	}

	res := &commentv1.ListCommentsResponse{
		Result: make([]*commentv1.CommentTree, len(trees)),
	}

	for idx, tree := range trees {
		res.Result[idx] = tree.ToProto(req.Msg.Recurse)

		minifyTree(res.Result[idx], true)
	}

	return connect.NewResponse(res), nil
}

func minifyTree(pb *commentv1.CommentTree, first bool) {
	pruneKeys := []string{
		"scope",
		"parent_id",
		"reference",
	}

	if !first {
		fmutils.Prune(pb.Comment, pruneKeys)
	}

	for _, answer := range pb.Answers {
		minifyTree(answer, false)
	}
}

func (svc *Service) parseAndRenderMarkDown(ctx context.Context, content string) (
	rootNode ast.Node,
	htmlContent string,
	userMentions []*idmv1.Profile,
	err error,
) {

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&mentions.Extender{
				Context: ctx,
				Resolver: mentions.ResolverFunc(
					func(n *mentions.Node) (*idmv1.Profile, error) {
						res, err := svc.Users.GetUser(ctx, connect.NewRequest(&idmv1.GetUserRequest{
							Search: &idmv1.GetUserRequest_Id{
								Id: string(n.Tag),
							},
						}))

						if err != nil {
							log.L(ctx).Debugf("failed to find user by id %q, trying by name", string(n.Tag))

							var cerr *connect.Error
							if !errors.As(err, &cerr) || cerr.Code() != connect.CodeNotFound {
								log.L(ctx).Infof("failed to get user by id: %q: %s", string(n.Tag), err)

								return nil, err
							}

							res, err = svc.Users.GetUser(ctx, connect.NewRequest(&idmv1.GetUserRequest{
								Search: &idmv1.GetUserRequest_Name{
									Name: string(n.Tag),
								},
							}))
						}

						if err != nil {
							log.L(ctx).Errorf("failed to get user by name or id: %q: %s", string(n.Tag), err)

							return nil, err
						}

						return res.Msg.GetProfile(), nil
					},
				),
			},
		),
	)

	rootNode = md.Parser().Parse(text.NewReader([]byte(content)))

	// Collect all users that are mentioned in the comment
	userMentionsMap := make(map[string]*idmv1.Profile)
	ast.Walk(rootNode, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
		if n, ok := node.(*mentions.Node); ok && enter && n.Profile != nil {
			userMentionsMap[n.Profile.User.Id] = n.Profile
		}

		return ast.WalkContinue, nil
	})

	// conver the userMentionsMap to a slice of *idmv1.Profile
	userMentions = data.MapToSlice(userMentionsMap)

	// actually render the markdown content as HTML
	buf := new(bytes.Buffer)
	if err := md.Renderer().Render(buf, []byte(content), rootNode); err != nil {
		return rootNode, "", userMentions, err
	}

	return rootNode, buf.String(), userMentions, nil
}

func (svc *Service) renderCommentInline(ctx context.Context, comment *models.Comment) error {
	_, htmlContent, _, err := svc.parseAndRenderMarkDown(ctx, comment.Content)
	if err != nil {
		return err
	}

	comment.Content = htmlContent

	return nil
}

func (svc *Service) renderCommentTree(ctx context.Context, tree *models.CommentTree) error {
	merr := new(multierror.Error)
	if err := svc.renderCommentInline(ctx, &tree.Comment); err != nil {
		merr.Errors = append(merr.Errors, fmt.Errorf("%q: %w", tree.Comment.ID.Hex(), err))
	}

	for idx := range tree.Answers {
		subTree := tree.Answers[idx]

		if err := svc.renderCommentTree(ctx, subTree); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}

	return merr.ErrorOrNil()
}

func (svc *Service) sendNotifications(comment models.Comment) {
	commentIdStr := comment.ID.Hex()

	// 10 seconds should be enought for our usecase, just make sure we don't get stuck somewhere
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// get the creator user profile so we can construct a pretty mail header
	creator, err := svc.Users.GetUser(ctx, connect.NewRequest(&idmv1.GetUserRequest{
		Search: &idmv1.GetUserRequest_Id{
			Id: comment.CreatorID,
		},
	}))
	if err != nil {
		log.L(ctx).Errorf("failed to load comment creator profile %q: %s", comment.CreatorID, err)
		return
	}

	// find all parent comments so we know which users to notify
	parentComments, err := svc.Repository.GetParentComments(ctx, commentIdStr)
	if err != nil {
		log.L(ctx).Errorf("failed to get parent comments for id %q: %s", commentIdStr, err)

		return
	}

	// build a user-"notification reason" map indexed by user id
	userMap := make(map[string]string /* reason: mention/parent */)
	for _, pc := range parentComments {
		// those users are notified because the created/answered at a parent comment
		userMap[pc.CreatorID] = "parent"
	}

	// parse the markdown content, extract/resolve @-user-mentions and convert it to some
	// nice HTML
	_, htmlContent, userMentions, err := svc.parseAndRenderMarkDown(ctx, comment.Content)
	if err != nil {
		log.L(ctx).Errorf("failed to parse and render comment content: %s", err)

		return
	}

	// add all user-ids from @-mentions
	for _, user := range userMentions {
		userMap[user.User.Id] = "mention"
	}

	// get a pretty display name for the creator
	creatorDisplayName := creator.Msg.GetProfile().GetUser().GetDisplayName()
	if creatorDisplayName == "" {
		creatorDisplayName = creator.Msg.GetProfile().GetUser().GetUsername()
	}

	// Finally, send e-mail notifications to all users that somehow participated in the
	// conversation. This is one after another, errors are only logged.
	for userId, reason := range userMap {
		// do not send mails to the creator of the comment
		if userId == comment.CreatorID {
			continue
		}

		subject := creatorDisplayName + " hat auf deinen Kommentar geantwortet"

		if reason == "mention" {
			subject = creatorDisplayName + " hat dich in einem Kommentar erwähnt"
		}

		// FIXME(ppacher): render a nice e-mail template here
		req := &idmv1.SendNotificationRequest{
			Message: &idmv1.SendNotificationRequest_Email{
				Email: &idmv1.EMailMessage{
					Subject: subject,
					Body:    htmlContent,
				},
			},
			TargetUsers:  []string{userId},
			SenderUserId: creator.Msg.GetProfile().GetUser().GetId(),
		}

		_, err := svc.Notify.SendNotification(ctx, connect.NewRequest(req))
		if err != nil {
			log.L(ctx).Errorf("failed to send notification to user %q: %s", userId, err)
		}
	}
}
