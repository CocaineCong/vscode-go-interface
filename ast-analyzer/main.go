package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type InterfaceMethod struct {
	Name          string   `json:"name"`
	InterfaceName string   `json:"interfaceName"`
	Location      Location `json:"location"`
}

type Implementation struct {
	MethodName   string   `json:"methodName"`
	ReceiverType string   `json:"receiverType"`
	Location     Location `json:"location"`
}

type AnalysisResult struct {
	Interfaces      []InterfaceMethod `json:"interfaces"`
	Implementations []Implementation  `json:"implementations"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> <directory/file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands: find-implementations, find-interfaces, find-file-interfaces, find-file-implementations\n")
		os.Exit(1)
	}

	command := os.Args[1]
	target := os.Args[2]

	switch command {
	case "find-implementations":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s find-implementations <directory> <method-name>\n", os.Args[0])
			os.Exit(1)
		}
		methodName := os.Args[3]
		implementations := findImplementations(target, methodName)
		result := AnalysisResult{Implementations: implementations}
		output, _ := json.Marshal(result)
		fmt.Println(string(output))

	case "find-interfaces":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s find-interfaces <directory> <method-name>\n", os.Args[0])
			os.Exit(1)
		}
		methodName := os.Args[3]
		interfaces := findInterfaces(target, methodName)
		result := AnalysisResult{Interfaces: interfaces}
		output, _ := json.Marshal(result)
		fmt.Println(string(output))

	case "find-file-interfaces":
		// 分析单个文件中的接口方法
		interfaces := findFileInterfaces(target)
		result := AnalysisResult{Interfaces: interfaces}
		output, _ := json.Marshal(result)
		fmt.Println(string(output))

	case "find-file-implementations":
		// 分析单个文件中的方法实现
		implementations := findFileImplementations(target)
		result := AnalysisResult{Implementations: implementations}
		output, _ := json.Marshal(result)
		fmt.Println(string(output))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}

// 分析单个文件中的接口方法
func findFileInterfaces(filePath string) []InterfaceMethod {
	var interfaces []InterfaceMethod
	fset := token.NewFileSet()

	if !strings.HasSuffix(filePath, ".go") {
		return interfaces
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return interfaces
	}

	f, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return interfaces
	}

	// 遍历AST查找接口定义
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			// 检查是否是接口类型
			if interfaceType, ok := node.Type.(*ast.InterfaceType); ok {
				interfaceName := node.Name.Name
				// 遍历接口方法
				for _, method := range interfaceType.Methods.List {
					if len(method.Names) > 0 {
						methodName := method.Names[0].Name
						pos := fset.Position(method.Pos())
						interfaces = append(interfaces, InterfaceMethod{
							Name:          methodName,
							InterfaceName: interfaceName,
							Location: Location{
								File:   filePath,
								Line:   pos.Line - 1,
								Column: pos.Column - 1,
							},
						})
					}
				}
			}
		}
		return true
	})

	return interfaces
}

// 分析单个文件中的方法实现
func findFileImplementations(filePath string) []Implementation {
	var implementations []Implementation
	fset := token.NewFileSet()

	if !strings.HasSuffix(filePath, ".go") {
		return implementations
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return implementations
	}

	f, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return implementations
	}

	// 遍历AST查找方法实现
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			// 检查是否是方法（有接收者）
			if node.Recv != nil {
				methodName := node.Name.Name
				receiverType := getReceiverType(node.Recv)
				pos := fset.Position(node.Pos())

				implementations = append(implementations, Implementation{
					MethodName:   methodName,
					ReceiverType: receiverType,
					Location: Location{
						File:   filePath,
						Line:   pos.Line - 1,
						Column: pos.Column - 1,
					},
				})
			}
		}
		return true
	})

	return implementations
}

func findImplementations(directory, methodName string) []Implementation {
	var implementations []Implementation
	fset := token.NewFileSet()

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过vendor目录和隐藏目录
		if info.IsDir() && (strings.Contains(path, "vendor") || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil
		}

		// 遍历AST查找方法实现
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				// 检查是否是方法（有接收者）
				if node.Recv != nil && node.Name.Name == methodName {
					// 获取接收者类型
					receiverType := getReceiverType(node.Recv)
					pos := fset.Position(node.Pos())

					implementations = append(implementations, Implementation{
						MethodName:   methodName,
						ReceiverType: receiverType,
						Location: Location{
							File:   path,
							Line:   pos.Line - 1, // VS Code使用0基索引
							Column: pos.Column - 1,
						},
					})
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
	}

	return implementations
}

func findInterfaces(directory, methodName string) []InterfaceMethod {
	var interfaces []InterfaceMethod
	fset := token.NewFileSet()

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过vendor目录和隐藏目录
		if info.IsDir() && (strings.Contains(path, "vendor") || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil
		}

		// 遍历AST查找接口定义
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				// 检查是否是接口类型
				if interfaceType, ok := node.Type.(*ast.InterfaceType); ok {
					interfaceName := node.Name.Name
					// 遍历接口方法
					for _, method := range interfaceType.Methods.List {
						if len(method.Names) > 0 && method.Names[0].Name == methodName {
							pos := fset.Position(method.Pos())
							interfaces = append(interfaces, InterfaceMethod{
								Name:          methodName,
								InterfaceName: interfaceName,
								Location: Location{
									File:   path,
									Line:   pos.Line - 1,
									Column: pos.Column - 1,
								},
							})
						}
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
	}

	return interfaces
}

func getReceiverType(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}

	field := recv.List[0]
	switch t := field.Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return "*" + ident.Name
		}
	}
	return ""
}
