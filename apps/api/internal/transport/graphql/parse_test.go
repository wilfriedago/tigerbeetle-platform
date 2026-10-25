package graphql

import (
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func parseQuery(query string) (*ast.QueryDocument, error) {
	return parser.ParseQuery(&ast.Source{Input: query})
}
