package models

import (
	"time"

	commentv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NotificationType string

var (
	NotificationTypeUnspecified = NotificationType("")
	NotificationTypeSMS         = NotificationType("sms")
	NotificationTypeEMail       = NotificationType("email")
)

type (
	Scope struct {
		InternalID             primitive.ObjectID `bson:"_id"`
		ID                     string             `bson:"scopeId"`
		Name                   string             `bson:"name"`
		NotificationType       NotificationType   `bson:"notificationType"`
		CommentViewURLTemplate string             `bson:"viewUrlTemplate"`
	}

	Comment struct {
		Scope     string             `bson:"scopeId"`
		Reference string             `bson:"ref"`
		ID        primitive.ObjectID `bson:"_id"`
		Content   string             `bson:"content"`
		ParentID  primitive.ObjectID `bson:"parentId,omitempty"`
		CreatedAt time.Time          `bson:"createdAt"`
		CreatorID string             `bson:"creatorId"`
	}

	CommentTree struct {
		Comment Comment
		Answers []*CommentTree
	}
)

func (s Scope) ToProto() *commentv1.Scope {
	return &commentv1.Scope{
		Id:                     s.ID,
		Name:                   s.Name,
		NotifcationType:        NotificationTypeToProto(s.NotificationType),
		ViewCommentUrlTemplate: s.CommentViewURLTemplate,
	}
}

func (c Comment) ToProto() *commentv1.Comment {
	cpb := &commentv1.Comment{
		Scope:     c.Scope,
		Id:        c.ID.Hex(),
		Content:   c.Content,
		CreatedAt: timestamppb.New(c.CreatedAt),
		CreatorId: c.CreatorID,
		Reference: c.Reference,
	}

	if !c.ParentID.IsZero() {
		cpb.ParentId = c.ParentID.Hex()
	}

	return cpb
}

func (ct CommentTree) ToProto(recurse bool) *commentv1.CommentTree {
	tree := &commentv1.CommentTree{
		Comment: ct.Comment.ToProto(),
	}

	if !recurse {
		return tree
	}

	tree.Answers = make([]*commentv1.CommentTree, len(ct.Answers))
	for idx, answer := range ct.Answers {
		tree.Answers[idx] = answer.ToProto(recurse)
	}

	return tree
}

func NotificationTypeToProto(nt NotificationType) commentv1.NotificationType {
	switch nt {
	case NotificationTypeEMail:
		return commentv1.NotificationType_NOTIFICATION_TYPE_MAIL
	case NotificationTypeSMS:
		return commentv1.NotificationType_NOTIFICATION_TYPE_SMS
	default:
		return commentv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
	}
}

func NotifcationTypeFromProto(npb commentv1.NotificationType) NotificationType {
	switch npb {
	case commentv1.NotificationType_NOTIFICATION_TYPE_MAIL:
		return NotificationTypeEMail
	case commentv1.NotificationType_NOTIFICATION_TYPE_SMS:
		return NotificationTypeSMS
	default:
		return NotificationTypeUnspecified
	}
}
