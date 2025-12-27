package xql

import (
	"go/ast"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func loadPkgAtDir(t *testing.T, dir string) *packages.Package {
	cfg := &packages.Config{Mode: packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: dir}
	pkgs, err := packages.Load(cfg, "./")
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	return pkgs[0]
}

func findTableReturnExpr(t *testing.T, pkg *packages.Package) (ast.Expr, *ast.File) {
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil && fn.Name.Name == "Table" {
				// find first return
				if fn.Body == nil {
					continue
				}
				for _, stmt := range fn.Body.List {
					if r, ok := stmt.(*ast.ReturnStmt); ok && len(r.Results) > 0 {
						return r.Results[0], f
					}
				}
			}
		}
	}
	t.Fatalf("no Table() return found in package %s", pkg.PkgPath)
	return nil, nil
}

func TestEvalStringExpr_Literal(t *testing.T) {
	pkgdir := filepath.Join("testdata", "literal")
	pkg := loadPkgAtDir(t, pkgdir)
	expr, file := findTableReturnExpr(t, pkg)
	v, ok := evalStringExpr(expr, pkg, file)
	require.True(t, ok)
	require.Equal(t, "accounts", v)
}

func TestEvalStringExpr_ConcatConst(t *testing.T) {
	pkgdir := filepath.Join("testdata", "concat")
	pkg := loadPkgAtDir(t, pkgdir)
	expr, file := findTableReturnExpr(t, pkg)
	v, ok := evalStringExpr(expr, pkg, file)
	require.True(t, ok)
	require.Equal(t, "pre_table", v)
}

func TestEvalStringExpr_SelectorConst(t *testing.T) {
	pkgdir := filepath.Join("testdata", "selector")
	pkg := loadPkgAtDir(t, pkgdir)
	expr, file := findTableReturnExpr(t, pkg)
	v, ok := evalStringExpr(expr, pkg, file)
	require.True(t, ok)
	require.Equal(t, "test_helpers_table", v)
}

func TestComputeEntityVersion_OrderInvariant(t *testing.T) {
	m1 := EntityMeta{
		StructName: "A",
		TableName:  "t",
		Fields: []Field{
			{GoName: "A", GoType: "int64", Name: "a"},
			{GoName: "B", GoType: "string", Name: "b"},
		},
	}
	m2 := EntityMeta{
		StructName: "A",
		TableName:  "t",
		Fields: []Field{
			{GoName: "B", GoType: "string", Name: "b"},
			{GoName: "A", GoType: "int64", Name: "a"},
		},
	}
	v1 := computeEntityVersion(m1)
	v2 := computeEntityVersion(m2)
	require.Equal(t, v1, v2)
}

func TestComputeEntityVersion_ChangeDetected(t *testing.T) {
	m := EntityMeta{
		StructName: "A",
		TableName:  "t",
		Fields: []Field{
			{GoName: "A", GoType: "int64", Name: "a"},
			{GoName: "B", GoType: "string", Name: "b"},
		},
	}
	v1 := computeEntityVersion(m)
	// change a field type
	m.Fields[1].GoType = "int64"
	v2 := computeEntityVersion(m)
	require.NotEqual(t, v1, v2)
}
