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
	// 添加结束位置
	EndLocation Location `json:"endLocation"`
}

type Implementation struct {
	MethodName   string   `json:"methodName"`
	ReceiverType string   `json:"receiverType"`
	Location     Location `json:"location"`
	// 添加结束位置
	EndLocation Location `json:"endLocation"`
}

type AnalysisResult struct {
	Interfaces      []InterfaceMethod `json:"interfaces"`
	Implementations []Implementation  `json:"implementations"`
}

type PackageAnalysisResult struct {
	InterfaceImplementations map[string][]string `json:"interfaceImplementations"` // 接口名 -> 实现方法列表
	MethodToInterface        map[string]string   `json:"methodToInterface"`        // 方法名 -> 接口名
}

func analyzePackageInterfaces(packagePath string) PackageAnalysisResult {
	result := PackageAnalysisResult{
		InterfaceImplementations: make(map[string][]string),
		MethodToInterface:        make(map[string]string),
	}

	// 1. 扫描包中所有 .go 文件
	files, err := filepath.Glob(filepath.Join(packagePath, "*.go"))
	if err != nil {
		return result
	}

	// 2. 收集所有接口定义
	interfaces := make(map[string][]InterfaceMethod)
	implementations := make(map[string][]Implementation)

	for _, file := range files {
		fileInterfaces := findFileInterfaces(file)
		fileImplementations := findFileImplementations(file)

		for _, iface := range fileInterfaces {
			interfaces[iface.InterfaceName] = append(interfaces[iface.InterfaceName], iface)
		}

		for _, impl := range fileImplementations {
			key := impl.ReceiverType + "." + impl.MethodName
			implementations[key] = append(implementations[key], impl)
		}
	}

	// 3. 匹配接口和实现
	for interfaceName, methods := range interfaces {
		for _, method := range methods {
			// 查找匹配的实现
			for _, impls := range implementations {
				for _, impl := range impls {
					if impl.MethodName == method.Name {
						// 这里可以添加更复杂的签名匹配逻辑
						result.InterfaceImplementations[interfaceName] = append(
							result.InterfaceImplementations[interfaceName],
							impl.MethodName,
						)
						result.MethodToInterface[impl.MethodName] = interfaceName
					}
				}
			}
		}
	}

	return result
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
	// 添加新的命令处理
	case "analyze-package-interfaces":
		// 分析整个包的接口实现关系
		packagePath := target
		result := analyzePackageInterfaces(packagePath)
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
						startPos := fset.Position(method.Pos())
						// 不使用 method.End()，而是计算下一行的位置
						nextLinePos := Location{
							File:   filePath,
							Line:   startPos.Line, // 下一行（因为我们已经减了1，所以这里不再减）
							Column: 0,             // 行首
						}
						interfaces = append(interfaces, InterfaceMethod{
							Name:          methodName,
							InterfaceName: interfaceName,
							Location: Location{
								File:   filePath,
								Line:   startPos.Line - 1,
								Column: startPos.Column - 1,
							},
							// 将 CodeLens 放在方法定义的下一行
							EndLocation: nextLinePos,
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
		fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
		return implementations
	}

	f, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析文件失败: %v\n", err)
		return implementations
	}

	// 获取文件所在目录，用于查找同目录下的所有接口
	dir := filepath.Dir(filePath)
	fmt.Fprintf(os.Stderr, "搜索目录: %s\n", dir)
	allInterfaces := findAllInterfacesInDirectory(dir)
	fmt.Fprintf(os.Stderr, "找到 %d 个接口\n", len(allInterfaces))
	for i, methods := range allInterfaces {
		fmt.Fprintf(os.Stderr, "接口 %d 的方法: %v\n", i+1, methods)
	}
	// 收集当前文件中所有类型的方法
	typeMethods := make(map[string][]string)
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Recv != nil {
				methodName := node.Name.Name
				receiverType := getReceiverType(node.Recv)
				typeMethods[receiverType] = append(typeMethods[receiverType], methodName)
			}
		}
		return true
	})
	fmt.Fprintf(os.Stderr, "类型方法映射: %v\n", typeMethods)

	// 检查哪些类型完整且精确地实现了接口
	for receiverType, methods := range typeMethods {
		fmt.Fprintf(os.Stderr, "检查类型 %s 的方法: %v\n", receiverType, methods)
		for i, interfaceMethods := range allInterfaces {
			fmt.Fprintf(os.Stderr, "与接口 %d 的方法 %v 进行匹配\n", i+1, interfaceMethods)
			if isExactMatch(methods, interfaceMethods) {
				fmt.Fprintf(os.Stderr, "✅ 类型 %s 完全匹配接口 %d\n", receiverType, i+1)
				// 这个类型完整且精确地实现了接口，添加其所有方法
				ast.Inspect(f, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.FuncDecl:
						if node.Recv != nil {
							currentReceiverType := getReceiverType(node.Recv)
							if currentReceiverType == receiverType {
								methodName := node.Name.Name
								startPos := fset.Position(node.Pos())
								endPos := fset.Position(node.End())

								implementations = append(implementations, Implementation{
									MethodName:   methodName,
									ReceiverType: receiverType,
									Location: Location{
										File:   filePath,
										Line:   startPos.Line - 1,
										Column: startPos.Column - 1,
									},
									EndLocation: Location{
										File:   filePath,
										Line:   endPos.Line - 1,
										Column: endPos.Column - 1,
									},
								})
							}
						}
					}
					return true
				})
				break // 找到匹配的接口后跳出
			} else {
				fmt.Fprintf(os.Stderr, "❌ 类型 %s 不匹配接口 %d\n", receiverType, i+1)
			}
		}
	}

	return implementations
}

// 查找目录中所有接口的方法列表（递归扫描子目录）
func findAllInterfacesInDirectory(dir string) [][]string {
	var allInterfaces [][]string
	fmt.Fprintf(os.Stderr, "开始递归搜索目录: %s\n", dir)

	// 递归遍历目录及其子目录中的所有.go文件
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "访问路径失败 %s: %v\n", path, err)
			return nil // 忽略错误，继续处理其他文件
		}

		// 只处理.go文件
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// 跳过测试文件
		if strings.HasSuffix(path, "_test.go") {
			fmt.Fprintf(os.Stderr, "跳过测试文件: %s\n", path)
			return nil
		}

		fset := token.NewFileSet()
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "分析文件: %s\n", path)

		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil
		}

		// 查找接口定义
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				if interfaceType, ok := node.Type.(*ast.InterfaceType); ok {
					var methods []string
					for _, method := range interfaceType.Methods.List {
						if len(method.Names) > 0 {
							methods = append(methods, method.Names[0].Name)
						}
					}
					if len(methods) > 0 {
						allInterfaces = append(allInterfaces, methods)
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		// 如果递归遍历失败，回退到只扫描当前目录
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return allInterfaces
		}

		for _, file := range files {
			// 跳过测试文件
			if strings.HasSuffix(file, "_test.go") {
				continue
			}

			fset := token.NewFileSet()
			src, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
			if err != nil {
				continue
			}

			// 查找接口定义
			ast.Inspect(f, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.TypeSpec:
					if interfaceType, ok := node.Type.(*ast.InterfaceType); ok {
						var methods []string
						for _, method := range interfaceType.Methods.List {
							if len(method.Names) > 0 {
								methods = append(methods, method.Names[0].Name)
							}
						}
						if len(methods) > 0 {
							allInterfaces = append(allInterfaces, methods)
						}
					}
				}
				return true
			})
		}
	}

	return allInterfaces
}

// 检查方法列表是否完全匹配（顺序无关）
// 修改 isExactMatch 函数
func isExactMatch(typeMethods []string, interfaceMethods []string) bool {
	// 创建类型方法的映射
	typeMethodSet := make(map[string]bool)
	for _, method := range typeMethods {
		typeMethodSet[method] = true
	}

	// 检查接口的每个方法是否都在类型中存在
	for _, interfaceMethod := range interfaceMethods {
		if !typeMethodSet[interfaceMethod] {
			return false
		}
	}

	return true
}

// 完全重写 findImplementations 函数
func findImplementations(directory, methodName string) []Implementation {
	var implementations []Implementation

	// 1. 首先找到包含该方法的接口
	var targetInterface *InterfaceInfo
	allInterfaces := findAllInterfacesWithMethods(directory)

	for _, iface := range allInterfaces {
		for _, method := range iface.Methods {
			if method == methodName {
				targetInterface = &iface
				break
			}
		}
		if targetInterface != nil {
			break
		}
	}

	if targetInterface == nil {
		return implementations
	}

	// 2. 收集所有类型的方法实现
	allTypeMethods := collectAllTypeMethods(directory)

	// 3. 检查每个类型是否完整且精确地实现了接口
	for typeName, methods := range allTypeMethods {
		methodNames := make([]string, 0, len(methods))
		for name := range methods {
			methodNames = append(methodNames, name)
		}

		// 检查是否完整且精确实现
		if isExactMatch(methodNames, targetInterface.Methods) {
			// 只返回用户点击的特定方法的实现
			if methodInfo, exists := methods[methodName]; exists {
				implementations = append(implementations, Implementation{
					MethodName:   methodName,
					ReceiverType: typeName,
					Location:     methodInfo.Location,
					EndLocation:  methodInfo.EndLocation,
				})
			}
		}
	}

	return implementations
}

// 接口信息结构
type InterfaceInfo struct {
	Name    string
	Methods []string
}

// 查找所有接口及其方法
func findAllInterfacesWithMethods(directory string) []InterfaceInfo {
	var interfaces []InterfaceInfo
	fset := token.NewFileSet()

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

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

		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				if interfaceType, ok := node.Type.(*ast.InterfaceType); ok {
					interfaceName := node.Name.Name
					var methods []string
					for _, method := range interfaceType.Methods.List {
						if len(method.Names) > 0 {
							methods = append(methods, method.Names[0].Name)
						}
					}
					interfaces = append(interfaces, InterfaceInfo{
						Name:    interfaceName,
						Methods: methods,
					})
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		// fmt.Printf("查找接口时出错: %v\n", err)
	}

	return interfaces
}

// 收集所有类型的方法
func collectAllTypeMethods(directory string) map[string]map[string]*MethodInfo {
	allTypeMethods := make(map[string]map[string]*MethodInfo)
	fset := token.NewFileSet()

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

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

		collectTypeMethods(f, fset, allTypeMethods)

		return nil
	})

	if err != nil {
		// fmt.Printf("收集方法时出错: %v\n", err)
	}

	return allTypeMethods
}

// 方法信息结构
type MethodInfo struct {
	Location    Location
	EndLocation Location
	FuncDecl    *ast.FuncDecl
}

// 收集类型的所有方法
func collectTypeMethods(f *ast.File, fset *token.FileSet, allTypeMethods map[string]map[string]*MethodInfo) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Recv != nil {
				receiverType := getReceiverType(node.Recv)
				if allTypeMethods[receiverType] == nil {
					allTypeMethods[receiverType] = make(map[string]*MethodInfo)
				}

				pos := fset.Position(node.Pos())
				endPos := fset.Position(node.End())

				allTypeMethods[receiverType][node.Name.Name] = &MethodInfo{
					Location: Location{
						File:   pos.Filename,
						Line:   pos.Line,
						Column: pos.Column,
					},
					EndLocation: Location{
						File:   endPos.Filename,
						Line:   endPos.Line,
						Column: endPos.Column - 1,
					},
					FuncDecl: node,
				}
			}
		}
		return true
	})
}

// 检查是否完整且精确地实现了接口
func isCompleteAndExactImplementation(typeMethods map[string]*MethodInfo, interfaceMethods []string) bool {
	// fmt.Printf("检查实现完整性和精确性:\n")
	// fmt.Printf("接口要求的方法: %v\n", interfaceMethods)
	typeMethodNames := make([]string, 0, len(typeMethods))
	for name := range typeMethods {
		typeMethodNames = append(typeMethodNames, name)
	}
	// fmt.Printf("类型实现的方法: %v\n", typeMethodNames)

	// 1. 完整性检查：必须实现接口的所有方法（方法名必须完全匹配）
	for _, ifaceMethod := range interfaceMethods {
		if _, exists := typeMethods[ifaceMethod]; !exists {
			// fmt.Printf("❌ 类型缺少接口方法: %s\n", ifaceMethod)
			return false // 缺少接口方法
		}
	}

	// 2. 精确性检查：方法数量必须完全匹配
	if len(typeMethods) != len(interfaceMethods) {
		// fmt.Printf("❌ 方法数量不匹配: 类型有 %d 个方法，接口需要 %d 个方法\n", len(typeMethods), len(interfaceMethods))
		return false
	}

	// 3. 严格匹配：确保所有方法都属于接口（方法名完全一致）
	for methodName := range typeMethods {
		found := false
		for _, ifaceMethod := range interfaceMethods {
			if methodName == ifaceMethod {
				found = true
				break
			}
		}
		if !found {
			// fmt.Printf("❌ 类型有额外的非接口方法: %s\n", methodName)
			return false // 有额外的非接口方法
		}
	}

	// fmt.Printf("✅ 类型完整且精确地实现了接口\n")
	return true
}

// 辅助函数：检查切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// 获取所有接口定义（保持不变）
func findAllInterfaces(directory string) []InterfaceMethod {
	var allInterfaces []InterfaceMethod

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && (strings.Contains(path, "vendor") || strings.HasPrefix(info.Name(), ".")) {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		interfaces := findFileInterfaces(path)
		allInterfaces = append(allInterfaces, interfaces...)

		return nil
	})

	if err != nil {
		// fmt.Printf("查找接口时出错: %v\n", err)
	}

	return allInterfaces
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
		// fmt.Printf("Error walking directory: %v\n", err)
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
