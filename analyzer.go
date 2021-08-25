package ireturn

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const nolintPrefix = "//nolint" // used for dissallow comments

const name string = "ireturn"

func NewAnalyzerWithConfig(config Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     name,
		Doc:      "Accept Interfaces, Return Concrete Types",
		Run:      run(config),
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func NewAnalyzer() *analysis.Analyzer {
	return NewAnalyzerWithConfig(NewDefaultConfig())
}

func run(config Config) func(*analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		var issues []analysis.Diagnostic

		ins, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
		ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
			//
			f, _ := node.(*ast.FuncDecl)

			// 02. Does it return any results ?
			if f.Type == nil || f.Type.Results == nil {
				return
			}

			// 02. Is it allowed to be checked?
			if hasDisallowDirective(f.Doc) {
				return
			}

			for _, i := range filterInterfaces(pass, f.Type.Results) {
				issues = append(issues, analysis.Diagnostic{
					Pos:     f.Pos(),
					Message: fmt.Sprintf("%s returns interface (%s)", f.Name.Name, i.name),
				})
			}
		})

		for i := range issues {
			pass.Report(issues[i])
		}

		return nil, nil
	}
}

func filterInterfaces(pass *analysis.Pass, fl *ast.FieldList) []iface {
	var results []iface

	for pos, el := range fl.List {
		switch v := el.Type.(type) {
		// -----
		case *ast.InterfaceType:

			if len(v.Methods.List) == 0 {
				results = append(results, issue("interface{}", pos, typeEmptyInterface))
				continue
			}

			results = append(results, issue("anonymouse interface", pos, typeAnonInterface))

		case *ast.Ident:

			t1 := pass.TypesInfo.TypeOf(el.Type)
			if !types.IsInterface(t1.Underlying()) {
				continue
			}

			word := t1.String()

			// only build in interface is error
			if obj := types.Universe.Lookup(word); obj != nil {
				results = append(results, issue(obj.Name(), pos, typeErrorInterface))
				continue
			}

			results = append(results, issue(word, pos, typeNamedInterface))

		case *ast.SelectorExpr:

			t1 := pass.TypesInfo.TypeOf(el.Type)
			if !types.IsInterface(t1.Underlying()) {
				continue
			}

			results = append(results, issue(t1.String(), pos, typeNamedInterface))

		// -----
		default:
			_ = v

		}
	}

	return results
}

func hasDisallowDirective(cg *ast.CommentGroup) bool {
	if cg == nil {
		return false
	}

	return disallowDirectiveFound(cg)
}

func disallowDirectiveFound(cg *ast.CommentGroup) bool {
	for i := len(cg.List) - 1; i >= 0; i-- {
		comment := cg.List[i]
		if !strings.HasPrefix(comment.Text, nolintPrefix) {
			continue
		}

		startingIdx := len(nolintPrefix)
		for {
			idx := strings.Index(comment.Text[startingIdx:], name)
			if idx == -1 {
				break
			}

			if len(comment.Text[startingIdx+idx:]) == len(name) {
				return true
			}

			c := comment.Text[startingIdx+idx+len(name)]
			if c == '.' || c == ',' || c == ' ' || c == '	' {
				return true
			}
			startingIdx += idx + 1
		}
	}

	return false
}
