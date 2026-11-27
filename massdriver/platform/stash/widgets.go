package stash

import (
	"context"
	"fmt"
	"iter"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/paging"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// WidgetDefinition is one version of a widget in the catalog — alias of
// [types.WidgetDefinition].
type WidgetDefinition = types.WidgetDefinition

// Widget is a widget drawn from a well-known key — alias of
// [types.Widget].
type Widget = types.Widget

// WidgetsInput controls a [Service.IterWidgetsForScopes] or
// [Service.WidgetsForScopes] call.
type WidgetsInput struct {
	// PageSize bounds the keys read per page, so a page can hold fewer
	// widgets than PageSize, or none.
	PageSize int
	// After is the opaque cursor from a prior [types.Page].Next. Pass the
	// same scopes as the call that returned the cursor.
	After string
}

// WidgetCatalog returns every version of every widget that the Massdriver
// UI draws from a well-known stash key, ordered by name and then version.
func (s *Service) WidgetCatalog(ctx context.Context) ([]WidgetDefinition, error) {
	resp, err := gen.GetWidgetCatalog(ctx, s.client.GQLv2)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get widget catalog: %w", err))
	}
	widgets := []WidgetDefinition{}
	if err := decode.Decode(resp.AlphaWidgetCatalog, &widgets); err != nil {
		return nil, fmt.Errorf("decode widget catalog: %w", err)
	}
	for i := range widgets {
		lowerScopeKind(&widgets[i].Scope)
	}
	return widgets, nil
}

// IterWidgetsForScopes returns a lazy [iter.Seq2] over the widgets in
// several scopes at once. Pass from 1 to 200 scopes, all in one project.
// When a scope has more than one version of a widget's key, the widget
// comes from the highest version.
func (s *Service) IterWidgetsForScopes(ctx context.Context, scopes []Scope, input WidgetsInput) iter.Seq2[Widget, error] {
	return paging.Iter(ctx, input.After, s.widgetsPage(scopes, input))
}

// WidgetsForScopes returns a single page of [Service.IterWidgetsForScopes].
// Keep reading pages until Next is empty: a page can hold no widgets and
// still have a next page.
func (s *Service) WidgetsForScopes(ctx context.Context, scopes []Scope, input WidgetsInput) (types.Page[Widget], error) {
	return s.widgetsPage(scopes, input)(ctx, input.After)
}

func (s *Service) widgetsPage(scopes []Scope, input WidgetsInput) paging.FetchFunc[Widget] {
	return withScopeKinds(paging.DecodeFetch[Widget](input.PageSize, "widget", func(ctx context.Context, cursor *scalars.Cursor) (paging.RawPage, error) {
		gs, err := toGenScopes(scopes)
		if err != nil {
			return paging.RawPage{}, err
		}
		resp, err := gen.ListWidgetsForScopes(ctx, s.client.GQLv2, s.client.Config.OrganizationID, gs, cursor)
		if err != nil {
			return paging.RawPage{}, gql.ClassifyError(fmt.Errorf("list widgets in %d scopes: %w", len(scopes), err))
		}
		return paging.RawPage{
			Items:    resp.AlphaWidgetsForScopes.Items,
			Next:     resp.AlphaWidgetsForScopes.Cursor.Next,
			Previous: resp.AlphaWidgetsForScopes.Cursor.Previous,
		}, nil
	}), func(w *Widget) *ScopeKind { return &w.Scope.Kind })
}
