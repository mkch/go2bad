// Package comments trims comments in AST.
package comments

import (
	"go/ast"
	"regexp"
	"slices"
)

// https://tip.golang.org/doc/comment#syntax (without line directive)
var reDirective = regexp.MustCompile(`^//(extern |export |[a-z0-9]+:[a-z0-9])`)

// https://pkg.go.dev/cmd/compile#hdr-Compiler_Directives
var reLineDirective = regexp.MustCompile(`^(//|/\*)line .*:.*$`)

func isDirective(comment string) bool {
	return reDirective.MatchString(comment) || reLineDirective.MatchString(comment)
}

// trimNodeComment trims all non-directive comments in *nodeComment.
// If nodeComment has an empty List after trimming, nil will be returned.
func trimNodeComment(nodeComment *ast.CommentGroup) *ast.CommentGroup {
	if nodeComment == nil {
		return nil
	}
	nodeComment.List = slices.DeleteFunc(nodeComment.List, func(c *ast.Comment) bool { return !isDirective(c.Text) })
	if len(nodeComment.List) == 0 {
		return nil
	}
	return nodeComment
}

// trimFileComments trims all non-directive comments in file
func trimFileComments(file *ast.File) {
	for i, comment := range file.Comments {
		if len(comment.List) == 0 {
			file.Comments[i] = nil
			continue
		}
		file.Comments[i] = trimNodeComment(comment)
	}
	file.Comments = slices.DeleteFunc(file.Comments, func(c *ast.CommentGroup) bool { return c == nil })
}

// Trim trims all comment nodes except directives.
func Trim(file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.File:
			node.Doc = trimNodeComment(node.Doc)
		case *ast.Field:
			node.Doc = trimNodeComment(node.Doc)
			node.Comment = trimNodeComment(node.Comment)
		case *ast.FuncDecl:
			node.Doc = trimNodeComment(node.Doc)
		case *ast.GenDecl:
			node.Doc = trimNodeComment(node.Doc)
		case *ast.ImportSpec:
			node.Doc = trimNodeComment(node.Doc)
			node.Comment = trimNodeComment(node.Comment)
		case *ast.TypeSpec:
			node.Doc = trimNodeComment(node.Doc)
			node.Comment = trimNodeComment(node.Comment)
		case *ast.ValueSpec:
			node.Doc = trimNodeComment(node.Doc)
			node.Comment = trimNodeComment(node.Comment)
		}
		return true
	})

	trimFileComments(file)
}
