package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"errors"

	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/generated"
	"github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/model"
)

// FindLibraryByID is the resolver for the findLibraryByID field.
func (r *entityResolver) FindLibraryByID(ctx context.Context, id string) (*model.Library, error) {
	return nil, errors.New("not implemented")
}

// FindPasswordAccountByEmail is the resolver for the findPasswordAccountByEmail field.
func (r *entityResolver) FindPasswordAccountByEmail(ctx context.Context, email string) (*model.PasswordAccount, error) {
	return nil, errors.New("not implemented")
}

// FindSMSAccountByNumber is the resolver for the findSMSAccountByNumber field.
func (r *entityResolver) FindSMSAccountByNumber(ctx context.Context, number *string) (*model.SMSAccount, error) {
	return nil, errors.New("not implemented")
}

// FindUserByID is the resolver for the findUserByID field.
func (r *entityResolver) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}

	return nil, nil
}

// FindUserByUsernameAndNameFirstAndNameLast is the resolver for the findUserByUsernameAndNameFirstAndNameLast field.
func (r *entityResolver) FindUserByUsernameAndNameFirstAndNameLast(ctx context.Context, username *string, nameFirst *string, nameLast *string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

// Entity returns generated.EntityResolver implementation.
func (r *Resolver) Entity() generated.EntityResolver { return &entityResolver{r} }

type entityResolver struct{ *Resolver }
