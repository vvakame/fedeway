package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"time"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/model"
)

func (r *libraryResolver) UserAccount(ctx context.Context, obj *model.Library, id string) (*model.User, error) {
	libraryUserIDs := r.libraryUsers[*obj.Name]
	for _, libraryUserID := range libraryUserIDs {
		if libraryUserID == id {
			return r.RootQuery().User(ctx, id)
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

func (r *rootQueryResolver) User(ctx context.Context, id string) (*model.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}

	return nil, nil
}

func (r *rootQueryResolver) Me(ctx context.Context) (*model.User, error) {
	return r.users[0], nil
}

func (r *userResolver) BirthDate(ctx context.Context, obj *model.User, locale *string) (*string, error) {
	if locale != nil && *locale != "" {
		loc, err := time.LoadLocation("Asia/Samarkand") // UTC + 5
		if err != nil {
			return nil, err
		}
		t, err := time.ParseInLocation("2006-01-02", *obj.BirthDate, loc)
		if err != nil {
			return nil, err
		}
		// TODO format with locale
		s := t.Format("2006-01-02")

		return &s, nil
	}

	return obj.BirthDate, nil
}

func (r *userResolver) Metadata(ctx context.Context, obj *model.User) ([]*model.UserMetadata, error) {
	return r.metadata[obj.ID], nil
}

// Library returns generated.LibraryResolver implementation.
func (r *Resolver) Library() generated.LibraryResolver { return &libraryResolver{r} }

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// RootQuery returns generated.RootQueryResolver implementation.
func (r *Resolver) RootQuery() generated.RootQueryResolver { return &rootQueryResolver{r} }

// User returns generated.UserResolver implementation.
func (r *Resolver) User() generated.UserResolver { return &userResolver{r} }

type libraryResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type rootQueryResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
