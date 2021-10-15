//go:generate go run github.com/99designs/gqlgen

package graph

import "github.com/vvakame/fedeway/internal/engine/subgraphs/accounts/graph/model"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	users        []*model.User
	metadata     map[string][]*model.UserMetadata
	libraryUsers map[string][]string
}

func NewResolver() *Resolver {
	strptr := func(s string) *string {
		return &s
	}

	users := []*model.User{
		{
			ID: "1",
			Name: &model.Name{
				First: strptr("Ada"),
				Last:  strptr("Lovelace"),
			},
			BirthDate: strptr("1815-12-10"),
			Username:  strptr("@ada"),
			Account: &model.PasswordAccount{ // TODO LibraryAccount?
				Email: "ada@example.com",
			},
			Ssn: strptr("123-45-6789"),
		},
		{
			ID: "2",
			Name: &model.Name{
				First: strptr("Alan"),
				Last:  strptr("Turing"),
			},
			BirthDate: strptr("1912-06-23"),
			Username:  strptr("@complete"),
			Account: &model.SMSAccount{
				Number: strptr("8675309"),
			},
			Ssn: strptr("987-65-4321"),
		},
	}

	metadata := map[string][]*model.UserMetadata{
		"1": {
			{
				Name:        strptr("meta1"),
				Address:     strptr("1"),
				Description: strptr("2"),
			},
		},
		"2": {
			{
				Name:        strptr("meta2"),
				Address:     strptr("3"),
				Description: strptr("4"),
			},
		},
	}

	libraryUsers := map[string][]string{
		"NYC Public Library": {"1", "2"},
	}

	return &Resolver{
		users:        users,
		metadata:     metadata,
		libraryUsers: libraryUsers,
	}
}
