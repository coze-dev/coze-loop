// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudwego/thriftgo/generator/golang"
	"github.com/cloudwego/thriftgo/generator/golang/streaming"
	"github.com/cloudwego/thriftgo/generator/golang/styles"
	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/semantic"
	"github.com/samber/lo"
	"golang.org/x/tools/imports"
)

type Generator struct {
	tf       *ThriftFile
	ns       string
	ast      *parser.Thrift
	resolver *resolver
}

func NewGenerator(tf *ThriftFile) (*Generator, error) {
	ast, err := parser.ParseFile(tf.IDL, tf.IncludeDirs, true)
	if err != nil {
		return nil, fmt.Errorf("parse thrift file fail, %s: %w", tf.IDL, err)
	}
	if err := semantic.ResolveSymbols(ast); err != nil {
		return nil, fmt.Errorf("semantic resolve symbols fail: %w", err)
	}
	ns, ok := ast.GetNamespace("go")
	if !ok {
		return nil, fmt.Errorf("no namespace for go: %s", tf.IDL)
	}

	return &Generator{
		tf:       tf,
		ast:      ast,
		ns:       strings.ReplaceAll(ns, ".", "/"),
		resolver: newResolver(tf.GoMod),
	}, nil
}

type ThriftFile struct {
	IncludeDirs           []string
	IDL                   string
	GoMod                 string
	KitexPrefix           string // kitex_gen/
	ImportPrefix          string // e.g. loop_gen/
	PackagePrefix         string // e.g. lo
	OutputDir             string // e.g. ./output
	LocalStreamImportPath string
}

func (g *Generator) Generate() error {
	pkg, err := g.solvePkg()
	if err != nil {
		return err
	}

	var errs []error
	svcPkg := g.resolver.ResolvePackage(g.ast, "")
	resolver := g.resolver
	ast := g.ast
	services := lo.Filter(ast.Services, func(s *parser.Service, _ int) bool { return s.Extends == "" })

	for _, service := range services {
		ssFuncs, err := g.filterServerStreamingFuncs(service)
		if err != nil {
			errs = append(errs, fmt.Errorf("filter server streaming fns fail, %s: %w", service.Name, err))
			continue
		}

		svcName := identifyThriftName(service.Name)
		kxClientPkg := strings.ToLower(svcName)
		kxClientImportPath := filepath.Join(g.tf.GoMod, g.tf.KitexPrefix, g.ns, kxClientPkg)
		s := &Schema{
			Package:            pkg,
			ServicePkg:         svcPkg,
			ServiceType:        svcName,
			ClientType:         fmt.Sprintf("Local%s", svcName),
			HasServerStreaming: len(ssFuncs) > 0,
		}
		s.ImportPath = filepath.Join(g.tf.GoMod, pkg)

		if s.HasServerStreaming {
			resolver.AddImport(g.tf.LocalStreamImportPath)
			resolver.AddImport(`fmt`) // print errorf
			resolver.AddImport(`github.com/cloudwego/kitex/client/callopt/streamcall`)
			resolver.AddImport(`github.com/cloudwego/kitex/pkg/streaming`)
			resolver.AddImport(kxClientImportPath)
		}

		for _, fn := range service.Functions {
			name := identifyThriftName(fn.Name)
			fnSchema := &Function{
				Name:            name,
				Comments:        fn.ReservedComments,
				Void:            fn.Void,
				ServerStreaming: ssFuncs[fn.Name],
				ArgType:         fmt.Sprintf("%s.%s%sArgs", svcPkg, svcName, strings.TrimSuffix(name, "_")),
				ResultType:      fmt.Sprintf("%s.%s%sResult", svcPkg, svcName, strings.TrimSuffix(name, "_")),
			}

			in := make([]string, 1, len(fn.Arguments)+2)
			args := make([]string, 1, cap(in)-1)
			in[0] = "ctx context.Context"
			args[0] = "ctx"

			for _, arg := range fn.Arguments {
				ident := resolver.ResolveType(ast, arg.Type)
				if golang.NeedRedirect(arg) && !strings.HasPrefix(ident, "*") {
					ident = "*" + ident
				}
				in = append(in, fmt.Sprintf("%s %s", arg.Name, ident))
				args = append(args, arg.Name)
				fnSchema.ReqFieldInArg = identifyThriftName(arg.Name)
				fnSchema.ReqNameInArg = arg.Name
			}

			var out string
			switch {
			case fnSchema.Void:
				out = "error"
				in = append(in, `callOptions ...callopt.Option`)
			case fnSchema.ServerStreaming:
				p := resolver.AddImport(kxClientImportPath)
				out = fmt.Sprintf(`stream %s.%s_%sClient, err error`, p, svcName, name)
				in = append(in, `callOptions ...streamcall.Option`)
				fnSchema.StreamRespIdent = resolver.ResolveType(ast, fn.FunctionType)
			default:
				out = resolver.ResolveType(ast, fn.FunctionType) + ", error"
				in = append(in, `callOptions ...callopt.Option`)
			}

			fnSchema.Out = out
			fnSchema.In = strings.Join(in, ", ")
			fnSchema.StubArgs = strings.Join(args, ", ")
			s.Functions = append(s.Functions, fnSchema)
		}

		s.Imports = resolver.AllImports()
		data, err := g.execGoTmpl(rpcTmpl, s)
		if err != nil {
			errs = append(errs, fmt.Errorf("generate service fail, %s: %w", svcName, err))
			continue
		}
		if err := g.writeFile(g.tf, s, data); err != nil {
			errs = append(errs, fmt.Errorf("write file %s.go fail: %w", svcName, err))
			continue
		}
	}

	var merr error
	for _, err := range errs {
		if err == nil {
			continue
		}
		if merr == nil {
			merr = err
		} else {
			merr = fmt.Errorf("%v; %w", merr, err)
		}
	}

	return merr
}

func (g *Generator) execGoTmpl(tmpl *template.Template, data interface{}) ([]byte, error) {
	var d bytes.Buffer
	if err := tmpl.Execute(&d, data); err != nil {
		return nil, fmt.Errorf("execute template fail, %s: %w", tmpl.Name(), err)
	}

	b, err := format.Source(d.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format Go code fail: %w", err)
	}

	// sort imports
	b, err = imports.Process("", b, &imports.Options{
		TabWidth:  8,
		TabIndent: true,
		Comments:  true,
		Fragment:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("format Go imports fail: %w", err)
	}
	return b, nil
}

func (g *Generator) solvePkg() (string, error) {
	pkg, _ := lo.Last(strings.Split(g.ns, "/"))
	pkg = fmt.Sprintf("%s%s", g.tf.PackagePrefix, pkg) // e.g. loprompt
	return pkg, nil
}

func (g *Generator) writeFile(tf *ThriftFile, s *Schema, data []byte) error {
	dir := filepath.Join(tf.OutputDir, tf.ImportPrefix, filepath.Dir(g.ns), s.Package)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	fullpath := filepath.Join(dir, fmt.Sprintf("local_%s.go", strings.ToLower(s.ServiceType)))
	return os.WriteFile(fullpath, data, 0644)
}

func (g *Generator) filterServerStreamingFuncs(s *parser.Service) (map[string]bool, error) {
	fns := make(map[string]bool)
	for _, fn := range s.Functions {
		stream, err := streaming.ParseStreaming(fn)
		if err != nil {
			return nil, fmt.Errorf("parse streaming fail, %s: %w", fn.Name, err)
		}
		if stream.ServerStreaming {
			fns[fn.Name] = true
		}
	}
	return fns, nil
}

var apacheNaming = new(styles.Apache)

func identifyThriftName(s string) string {
	name, _ := apacheNaming.Identify(s)
	if strings.HasPrefix(name, "New") {
		name += "_"
	}
	return name
}
