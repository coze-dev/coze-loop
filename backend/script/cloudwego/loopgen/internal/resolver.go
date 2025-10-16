// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudwego/thriftgo/parser"
)

var baseTypes = map[string]string{
	"bool":   "bool",
	"byte":   "int8",
	"i8":     "int8",
	"i16":    "int16",
	"i32":    "int32",
	"i64":    "int64",
	"double": "float64",
	"string": "string",
	"binary": "[]byte",
}

var rpcImports = []string{
	"context",
	"github.com/cloudwego/kitex/client/callopt",
	"github.com/cloudwego/kitex/pkg/endpoint",
	"github.com/cloudwego/kitex/pkg/rpcinfo",
}

type resolver struct {
	ipMap        map[*parser.Thrift]string /*import path*/
	pkgPrefix    string
	imports      *GoImportHelper
	resolveIdent func(*parser.Thrift, *parser.Type) string
}

func newResolver(pkgPrefix string) *resolver {
	r := &resolver{
		ipMap:     make(map[*parser.Thrift]string),
		pkgPrefix: makePackagePrefix(pkgPrefix),
		imports:   NewGoImportHelper(nil),
	}

	for _, ip := range rpcImports {
		r.imports.AddImport(ip)
	}

	r.buildResolver()
	return r
}

func (r *resolver) ResolveType(ast *parser.Thrift, t *parser.Type) string {
	return r.resolveIdent(ast, t)
}

func (r *resolver) ResolvePackage(ast *parser.Thrift, pkg string) (nameOrAlias string) {
	importPath := r.astImportPath(ast)
	return r.imports.AddImport(filepath.Join(importPath, pkg))
}

func (r *resolver) AllImports() []string {
	return r.imports.Imports()
}

func (r *resolver) AddImport(importPath string) string {
	return r.imports.AddImport(importPath)
}

func (r *resolver) buildResolver() {
	r.resolveIdent = func(ast *parser.Thrift, t *parser.Type) (ident string) {
		if t.Category.IsStructLike() {
			defer func() { ident = "*" + ident }()
		}

		switch {
		case t.Category.IsBaseType():
			ident = baseTypes[t.Name]

		case t.Category.IsContainerType():
			v := r.resolveIdent(ast, t.ValueType)

			if t.Category.IsMap() {
				var k string
				if t.KeyType.Category == parser.Category_Binary {
					k = "string"
				} else {
					k = r.resolveIdent(ast, t.KeyType)
				}

				ident = fmt.Sprintf("map[%s]%s", k, v)
			} else { // set or list
				ident = fmt.Sprintf("[]%s", v)
			}

		case t.GetReference() != nil:
			ref := t.GetReference()
			ast := ast.Includes[ref.Index].Reference
			ident = r.imports.QualifiedGoIdentWithAlias(GoIdent{
				GoName:       identifyThriftName(ref.Name),
				GoImportPath: r.astImportPath(ast),
			}, r.nsToPackage(ast.GetNamespaceOrReferenceName("go")))

		default: // types defined in current file
			ident = r.imports.QualifiedGoIdentWithAlias(GoIdent{
				GoName:       identifyThriftName(t.Name),
				GoImportPath: r.astImportPath(ast),
			}, r.nsToPackage(ast.GetNamespaceOrReferenceName("go")))
		}

		return
	}
}

func (r *resolver) astImportPath(ast *parser.Thrift) string {
	ip, ok := r.ipMap[ast]
	if !ok {
		ip = r.nsToImportPath(ast.GetNamespaceOrReferenceName("go"))
		r.ipMap[ast] = ip
	}
	return ip
}

func (r *resolver) nsToImportPath(ns string) string {
	pkg := strings.ReplaceAll(ns, ".", "/")
	return filepath.Join(r.pkgPrefix, pkg)
}

func (r *resolver) nsToPackage(ns string) string {
	parts := strings.Split(ns, ".")
	return strings.ToLower(parts[len(parts)-1])
}

func makePackagePrefix(pkg string) string {
	switch {
	case pkg == "":
	case strings.HasSuffix(pkg, "/kitex_gen"),
		strings.HasPrefix(pkg, "/kitex_gen/"):
	default:
		pkg = filepath.Join(pkg, "kitex_gen")
	}
	return pkg
}
