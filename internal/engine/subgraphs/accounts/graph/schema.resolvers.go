package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"time"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/model"
)

func (r *libraryResolver) UserAccount(ctx context.Context, obj *model.Library, id string) (*model.User, error) {
	libraryUserIDs := r.libraryUsers[*obj.Name]
	for _, libraryUserID := range libraryUserIDs {
		if libraryUserID == id {
			return r.Query().User(ctx, id)
		}
	}

	return nil, nil
}

func (r *mutationResolver) Login(ctx context.Context, username string, password string, userID *string) (*model.User, error) {
	for _, user := range r.users {
		if user.Username != nil && *user.Username == username {
			return user, nil
		}
	}

	return nil, nil
}

func (r *queryResolver) User(ctx context.Context, id string) (*model.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}

	return nil, nil
}

func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	return r.users[0], nil
}

func (r *userResolver) BirthDate(ctx context.Context, obj *model.User, locale *string) (*string, error) {
	if locale == nil || *locale == "" {
		return obj.BirthDate, nil
	}

	loc, err := time.LoadLocation("Asia/Samarkand") // UTC + 5
	if err != nil {
		return nil, err
	}
	t, err := time.ParseInLocation("2006-01-02", *obj.BirthDate, loc)
	if err != nil {
		return nil, err
	}

	switch *locale {
	case "en-US":
		s := t.Format("1/2/2006")
		return &s, nil
	default:
		return nil, fmt.Errorf("unknown locale: %s", *locale)
	}
}

func (r *userResolver) Metadata(ctx context.Context, obj *model.User) ([]*model.UserMetadata, error) {
	return r.metadata[obj.ID], nil
}

// Library returns generated.LibraryResolver implementation.
func (r *Resolver) Library() generated.LibraryResolver { return &libraryResolver{r} }

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// User returns generated.UserResolver implementation.
func (r *Resolver) User() generated.UserResolver { return &userResolver{r} }

type libraryResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
