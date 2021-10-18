// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"fmt"
	"io"
	"strconv"
)

type Brand interface {
	IsBrand()
}

type MetadataOrError interface {
	IsMetadataOrError()
}

type Product interface {
	IsProduct()
}

type ProductDetails interface {
	IsProductDetails()
}

type Thing interface {
	IsThing()
}

type Vehicle interface {
	IsVehicle()
}

type Amazon struct {
	Referrer *string `json:"referrer"`
}

func (Amazon) IsBrand() {}

type Book struct {
	Isbn    string              `json:"isbn"`
	Title   *string             `json:"title"`
	Year    *int                `json:"year"`
	Upc     string              `json:"upc"`
	Sku     string              `json:"sku"`
	Name    *string             `json:"name"`
	Price   *string             `json:"price"`
	Details *ProductDetailsBook `json:"details"`
}

func (Book) IsEntity()  {}
func (Book) IsProduct() {}

type Car struct {
	ID          string  `json:"id"`
	Description *string `json:"description"`
	Price       *string `json:"price"`
}

func (Car) IsVehicle() {}
func (Car) IsThing()   {}
func (Car) IsEntity()  {}

type Error struct {
	Code    *int    `json:"code"`
	Message *string `json:"message"`
}

func (Error) IsMetadataOrError() {}

type Furniture struct {
	Upc      string                   `json:"upc"`
	Sku      string                   `json:"sku"`
	Name     *string                  `json:"name"`
	Price    *string                  `json:"price"`
	Brand    Brand                    `json:"brand"`
	Metadata []MetadataOrError        `json:"metadata"`
	Details  *ProductDetailsFurniture `json:"details"`
}

func (Furniture) IsProduct() {}
func (Furniture) IsEntity()  {}

type Ikea struct {
	Asile *int `json:"asile"`
}

func (Ikea) IsBrand() {}
func (Ikea) IsThing() {}

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (KeyValue) IsMetadataOrError() {}

type ProductDetailsBook struct {
	Country *string `json:"country"`
	Pages   *int    `json:"pages"`
}

func (ProductDetailsBook) IsProductDetails() {}

type ProductDetailsFurniture struct {
	Country *string `json:"country"`
	Color   *string `json:"color"`
}

func (ProductDetailsFurniture) IsProductDetails() {}

type User struct {
	ID      string  `json:"id"`
	Vehicle Vehicle `json:"vehicle"`
	Thing   Thing   `json:"thing"`
}

func (User) IsEntity() {}

type Van struct {
	ID          string  `json:"id"`
	Description *string `json:"description"`
	Price       *string `json:"price"`
}

func (Van) IsVehicle() {}
func (Van) IsEntity()  {}

type CacheControlScope string

const (
	CacheControlScopePublic  CacheControlScope = "PUBLIC"
	CacheControlScopePrivate CacheControlScope = "PRIVATE"
)

var AllCacheControlScope = []CacheControlScope{
	CacheControlScopePublic,
	CacheControlScopePrivate,
}

func (e CacheControlScope) IsValid() bool {
	switch e {
	case CacheControlScopePublic, CacheControlScopePrivate:
		return true
	}
	return false
}

func (e CacheControlScope) String() string {
	return string(e)
}

func (e *CacheControlScope) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = CacheControlScope(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid CacheControlScope", str)
	}
	return nil
}

func (e CacheControlScope) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}