// 在文件顶部的变量声明部分，确保这两行存在：
import * as vscode from 'vscode';
import * as path from 'path';
import * as cp from 'child_process';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined; // 修改为可选类型
let interfaceDecorator: vscode.TextEditorDecorationType | undefined; // 添加这行
let implementationDecorator: vscode.TextEditorDecorationType | undefined; // 接口实现装饰器

// AST分析结果的类型定义
interface Location {
  file: string;
  line: number;
  column: number;
}

interface InterfaceMethod {
  name: string;
  interfaceName: string;
  location: Location;
  endLocation: Location;
}

interface Implementation {
  methodName: string;
  receiverType: string;
  location: Location;
}

interface AnalysisResult {
  interfaces?: InterfaceMethod[];
  implementations?: Implementation[];
}

// 获取AST分析器路径 - 修复路径问题
function getAstAnalyzerPath(): string {
  // 获取当前扩展的路径
  const extensionPath = vscode.extensions.getExtension('your-publisher.goimpl-vscode')?.extensionPath;
  if (extensionPath) {
    return path.join(extensionPath, 'ast-analyzer', 'ast-analyzer');
  }
  
  // 开发模式：使用固定的扩展开发路径
  return '/Users/mac/VscodeProjects/goimpl-vscode/ast-analyzer/ast-analyzer';
}

// 基于AST的CodeLens Provider
class GoInterfaceCodeLensProvider implements vscode.CodeLensProvider {
  public async provideCodeLenses(document: vscode.TextDocument): Promise<vscode.CodeLens[]> {
    if (document.languageId !== 'go') {
      return [];
    }

    const codeLenses: vscode.CodeLens[] = [];
    const filePath = document.uri.fsPath;
    const packagePath = path.dirname(document.fileName);
    const packageAnalysis = await this.analyzePackageInterfaces(packagePath);

    try {
      // 分析当前文件中的接口和实现
      const interfaces = await this.analyzeFileInterfaces(document.fileName);
      const implementations = await this.analyzeFileImplementations(document.fileName);
      this.addInterfaceDecorations(document);
      this.addImplementationDecorations(document);
      console.log('Found interfaces:', interfaces.length);
      console.log('Found implementations:', implementations.length);
        // 创建接口方法名称集合
      const interfaceMethodNames = new Set(interfaces.map(iface => iface.name));
      console.log('Interface method names:', Array.from(interfaceMethodNames));
       for (const interfaceMethod of interfaces) {
          const range = new vscode.Range(
            interfaceMethod.location.line,
            interfaceMethod.location.column,
            interfaceMethod.location.line,
            interfaceMethod.location.column + interfaceMethod.name.length
          );
      
      codeLenses.push(new vscode.CodeLens(range, {
        title: "🔍 implementations",
        command: "goInterfaceNavigator.findImplementations",
        arguments: [interfaceMethod.name]
      }));
    }
       // 分析方法实现（只显示完整且精确实现接口的方法）
      for (const impl of implementations) {
        if (packageAnalysis.methodToInterface[impl.methodName]) {
            const range = new vscode.Range(impl.location.line, impl.location.column, impl.location.line, impl.location.column);
            const codeLens = new vscode.CodeLens(range, {
                title: '✅ interface implementation',
                command: 'goInterfaceNavigator.findInterface',
                arguments: [impl.methodName]
            });
            codeLenses.push(codeLens);
        }
    }
    } catch (error) {
      console.error('AST分析错误:', error);
    }

  
    return codeLenses;
  }
  

  async checkInterfaceCompleteness(interfaceName: string, filePath: string): Promise<{isComplete: boolean, implementationCount: number, totalMethods: number}> {
    try {
      const workspaceRoot = vscode.workspace.getWorkspaceFolder(vscode.Uri.file(filePath))?.uri.fsPath;
      if (!workspaceRoot) {
        return {isComplete: false, implementationCount: 0, totalMethods: 0};
      }

      // 获取接口的所有方法
      const allInterfaces = await this.analyzeFileInterfaces(filePath);
      const interfaceMethods = allInterfaces.filter(iface => iface.interfaceName === interfaceName);
      const totalMethods = interfaceMethods.length;

      // 检查实现
      const implementations = await this.analyzeFileImplementations(filePath);
      const implementationCount = implementations.length;

      // 简单的完整性检查（可以根据需要改进）
      const isComplete = implementationCount > 0 && implementationCount === totalMethods;

      return {isComplete, implementationCount, totalMethods};
    } catch (error) {
      console.error('检查接口完整性时出错:', error);
      return {isComplete: false, implementationCount: 0, totalMethods: 0};
    }
  }

  async addInterfaceDecorations(document: vscode.TextDocument) {
    if (!interfaceDecorator) {
      return;
    }

    const interfaces = await this.analyzeFileInterfaces(document.uri.fsPath);
    if (!interfaces || interfaces.length === 0) {
      return;
    }

    const decorations: vscode.DecorationOptions[] = [];
    
    for (const interfaceMethod of interfaces) {
      const line = interfaceMethod.location.line; // 移除 -1
      const range = new vscode.Range(line, 0, line, 0);
      
      decorations.push({
        range,
        hoverMessage: `⚡️ 接口方法: ${interfaceMethod.interfaceName}.${interfaceMethod.name}`
      });
    }

    // 应用装饰器到当前活动编辑器
    const editor = vscode.window.activeTextEditor;
    if (editor && editor.document === document) {
      editor.setDecorations(interfaceDecorator, decorations);
    }
  }

  private async analyzePackageInterfaces(packagePath: string): Promise<any> {
    return new Promise((resolve, reject) => {
        const astAnalyzerPath = getAstAnalyzerPath();
        const command = `${astAnalyzerPath} analyze-package-interfaces "${packagePath}"`;
        cp.exec(command, (error, stdout, stderr) => {
            if (error) {
                console.error('Error analyzing package interfaces:', error);
                resolve({ interfaceImplementations: {}, methodToInterface: {} });
                return;
            }
            try {
                const result = JSON.parse(stdout);
                resolve(result);
            } catch (parseError) {
                console.error('Error parsing package analysis result:', parseError);
                resolve({ interfaceImplementations: {}, methodToInterface: {} });
            }
        });
    });
}

  private async analyzeFileInterfaces(filePath: string): Promise<InterfaceMethod[]> {
    return new Promise((resolve) => {
      const astAnalyzerPath = getAstAnalyzerPath();
      const command = `"${astAnalyzerPath}" find-file-interfaces "${filePath}"`;
      
      console.log('执行接口分析命令:', command);
      
      cp.exec(command, (error, stdout, stderr) => {
        if (error) {
          console.error('AST分析器错误:', error);
          console.error('stderr:', stderr);
          resolve([]);
          return;
        }
        
        console.log('接口分析原始输出:', stdout);
        
        try {
          const result: AnalysisResult = JSON.parse(stdout);
          console.log('解析后的接口结果:', result);
          console.log('接口数组:', result.interfaces);
          resolve(result.interfaces || []);
        } catch (parseError) {
          console.error('解析AST输出失败:', parseError);
          console.error('stdout:', stdout);
          resolve([]);
        }
      });
    });
  }

  

  private async analyzeFileImplementations(filePath: string): Promise<Implementation[]> {
    return new Promise((resolve) => {
      const astAnalyzerPath = getAstAnalyzerPath();
      const command = `"${astAnalyzerPath}" find-file-implementations "${filePath}"`;
      
      console.log('执行命令:', command); // 调试日志
      
      cp.exec(command, (error, stdout, stderr) => {
        if (error) {
          console.error('AST分析器错误:', error);
          console.error('stderr:', stderr);
          resolve([]);
          return;
        }
        
        try {
          const result: AnalysisResult = JSON.parse(stdout);
          resolve(result.implementations || []);
        } catch (parseError) {
          console.error('解析AST输出失败:', parseError);
          console.error('stdout:', stdout);
          resolve([]);
        }
      });
    });
  }

  async addImplementationDecorations(document: vscode.TextDocument) {
    if (!implementationDecorator) {
      return;
    }
    const packagePath = path.dirname(document.fileName);
    const packageAnalysis = await this.analyzePackageInterfaces(packagePath);
    const implementations = await this.analyzeFileImplementations(document.uri.fsPath);
    if (!implementations || implementations.length === 0) {
      return;
    }
    const decorations: vscode.DecorationOptions[] = [];
    for (const impl of implementations) {
      if (packageAnalysis.methodToInterface[impl.methodName]) {
        const line = impl.location.line; // 使用正确的行号（不减1）
        const range = new vscode.Range(line, 0, line, 0);
      
        decorations.push({
          range,
          hoverMessage: `🔧 接口实现: ${impl.receiverType}.${impl.methodName}`
        });
      }
    }

    // 应用装饰器到当前活动编辑器
    const editor = vscode.window.activeTextEditor;
    if (editor && editor.document === document) {
      editor.setDecorations(implementationDecorator, decorations);
    }
  }

}

export function activate(context: vscode.ExtensionContext) {

  interfaceDecorator = vscode.window.createTextEditorDecorationType({
    // 使用 gutterIconPath 在行号左边显示图标
    gutterIconPath: vscode.Uri.parse('data:image/svg+xml;base64,' + Buffer.from(`
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
        <text x="8" y="12" font-family="Arial" font-size="12" text-anchor="middle" fill="#569CD6">⚡️</text>
      </svg>
    `).toString('base64')),
    gutterIconSize: 'contain'
  });

  // 接口实现装饰器（新增的）
  implementationDecorator = vscode.window.createTextEditorDecorationType({
    gutterIconPath: vscode.Uri.parse('data:image/svg+xml;base64,' + Buffer.from(`
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
        <text x="8" y="12" font-family="Arial" font-size="12" text-anchor="middle" fill="#4EC9B0">🔧</text>
      </svg>
    `).toString('base64')),
    gutterIconSize: 'contain'
  });


  // 注册CodeLens提供者
  const codeLensProvider = new GoInterfaceCodeLensProvider();
  context.subscriptions.push(
    vscode.languages.registerCodeLensProvider({ language: 'go' }, codeLensProvider)
  );

  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(async (editor) => {
      if (editor && editor.document.languageId === 'go') {
        // 重新分析并应用装饰器
        await codeLensProvider.provideCodeLenses(editor.document);
      }
    })
  );

   // 仿照现有的事件监听器模式
  context.subscriptions.push(
    vscode.window.onDidChangeActiveTextEditor(async (editor) => {
      if (editor && editor.document.languageId === 'go') {
        // 重新分析并应用装饰器
        await codeLensProvider.provideCodeLenses(editor.document);
      }
    })
  );

  context.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument(async (event) => {
      const editor = vscode.window.activeTextEditor;
      if (editor && editor.document === event.document && event.document.languageId === 'go') {
        // 延迟更新，避免频繁调用
        setTimeout(async () => {
          await codeLensProvider.provideCodeLenses(event.document);
        }, 500);
      }
    })
  );

    context.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument(async (event) => {
      if (event.document.languageId === 'go') {
        // 延迟更新装饰器，避免频繁更新
        setTimeout(async () => {
          await codeLensProvider.provideCodeLenses(event.document);
        }, 500);
      }
    })
  );
  
  // 查找实现命令 - 简化参数处理
  const findImplementationsCommand = vscode.commands.registerCommand(
    'goInterfaceNavigator.findImplementations',
    async (...args: any[]) => {
      let methodName: string;
      let editor = vscode.window.activeTextEditor;
      
      console.log('findImplementations called with args:', args);
      
      // 处理不同的调用方式
      if (args.length > 0 && typeof args[0] === 'string' && !args[0].startsWith('file://')) {
        // 直接传入的方法名（来自我们的CodeLens）
        methodName = args[0];
      } else {
        // 从当前位置获取方法名
        if (!editor) {
          vscode.window.showErrorMessage('没有活动的编辑器');
          return;
        }
        
        const currentPosition = editor.selection.active;
        const wordRange = editor.document.getWordRangeAtPosition(currentPosition);
        if (!wordRange) {
          vscode.window.showErrorMessage('请将光标放在方法名上');
          return;
        }
        
        methodName = editor.document.getText(wordRange);
      }

      console.log('查找实现的方法名:', methodName);

      // 获取当前工作区的根目录
      const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
      if (!workspaceFolder) {
        vscode.window.showErrorMessage('未找到工作区文件夹');
        return;
      }

      try {
        const implementations = await findImplementationsWithAST(workspaceFolder.uri.fsPath, methodName);
        
        if (implementations.length === 0) {
          vscode.window.showInformationMessage(`未找到方法 "${methodName}" 的实现`);
          return;
        }

        const locations = implementations.map(impl => new vscode.Location(
          vscode.Uri.file(impl.location.file),
          new vscode.Position(impl.location.line - 1, impl.location.column)
        ));

        if (locations.length === 1) {
          // 直接跳转
          const location = locations[0];
          const doc = await vscode.workspace.openTextDocument(location.uri);
          const newEditor = await vscode.window.showTextDocument(doc);
          newEditor.selection = new vscode.Selection(location.range.start, location.range.start);
          newEditor.revealRange(location.range, vscode.TextEditorRevealType.InCenter);
        } else {
          // 显示引用面板
          if (editor) {
            await vscode.commands.executeCommand(
              'editor.action.showReferences',
              editor.document.uri,
              editor.selection.active,
              locations
            );
          }
        }
      } catch (error: any) {
        console.error('查找实现错误:', error);
        vscode.window.showErrorMessage(`查找实现时出错: ${error.message}`);
      }
    }
  );

  // 查找接口命令 - 简化参数处理
  const findInterfaceCommand = vscode.commands.registerCommand(
    'goInterfaceNavigator.findInterface',
    async (...args: any[]) => {
      let methodName: string;
      let editor = vscode.window.activeTextEditor;
      
      console.log('findInterface called with args:', args);
      
      // 处理不同的调用方式
      if (args.length > 0 && typeof args[0] === 'string' && !args[0].startsWith('file://')) {
        // 直接传入的方法名
        methodName = args[0];
      } else {
        // 从当前位置获取方法名
        if (!editor) {
          vscode.window.showErrorMessage('没有活动的编辑器');
          return;
        }
        
        const currentPosition = editor.selection.active;
        const wordRange = editor.document.getWordRangeAtPosition(currentPosition);
        if (!wordRange) {
          vscode.window.showErrorMessage('请将光标放在方法名上');
          return;
        }
        
        methodName = editor.document.getText(wordRange);
      }

      console.log('查找接口的方法名:', methodName);

      // 获取当前工作区的根目录
      const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
      if (!workspaceFolder) {
        vscode.window.showErrorMessage('未找到工作区文件夹');
        return;
      }

      try {
        const interfaces = await findInterfacesWithAST(workspaceFolder.uri.fsPath, methodName);
        
        if (interfaces.length === 0) {
          vscode.window.showInformationMessage(`未找到方法 "${methodName}" 的接口定义`);
          return;
        }

        const locations = interfaces.map(iface => new vscode.Location(
          vscode.Uri.file(iface.location.file),
          new vscode.Position(iface.location.line - 1, iface.location.column)
        ));

        if (locations.length === 1) {
          // 直接跳转
          const location = locations[0];
          const doc = await vscode.workspace.openTextDocument(location.uri);
          const newEditor = await vscode.window.showTextDocument(doc);
          newEditor.selection = new vscode.Selection(location.range.start, location.range.start);
          newEditor.revealRange(location.range, vscode.TextEditorRevealType.InCenter);
        } else {
          // 显示引用面板
          if (editor) {
            await vscode.commands.executeCommand(
              'editor.action.showReferences',
              editor.document.uri,
              editor.selection.active,
              locations
            );
          }
        }
      } catch (error: any) {
        console.error('查找接口错误:', error);
        vscode.window.showErrorMessage(`查找接口时出错: ${error.message}`);
      }
    }
  );
  
}

// 辅助函数
async function findImplementationsWithAST(directory: string, methodName: string): Promise<Implementation[]> {
  return new Promise((resolve) => {
    const astAnalyzerPath = getAstAnalyzerPath();
    const command = `"${astAnalyzerPath}" find-implementations "${directory}" "${methodName}"`;
    
    console.log('查找实现命令:', command); // 调试日志
    
    cp.exec(command, (error, stdout, stderr) => {
      if (error) {
        console.error('AST分析器错误:', error);
        console.error('stderr:', stderr);
        resolve([]);
        return;
      }
      
      try {
        const result: AnalysisResult = JSON.parse(stdout);
        resolve(result.implementations || []);
      } catch (parseError) {
        console.error('解析AST输出失败:', parseError);
        console.error('stdout:', stdout);
        resolve([]);
      }
    });
  });
}

async function findInterfacesWithAST(directory: string, methodName: string): Promise<InterfaceMethod[]> {
  return new Promise((resolve) => {
    const astAnalyzerPath = getAstAnalyzerPath();
    const command = `"${astAnalyzerPath}" find-interfaces "${directory}" "${methodName}"`;
    
    console.log('查找接口命令:', command); // 调试日志
    
    cp.exec(command, (error, stdout, stderr) => {
      if (error) {
        console.error('AST分析器错误:', error);
        console.error('stderr:', stderr);
        resolve([]);
        return;
      }
      
      try {
        const result: AnalysisResult = JSON.parse(stdout);
        resolve(result.interfaces || []);
      } catch (parseError) {
        console.error('解析AST输出失败:', parseError);
        console.error('stdout:', stdout);
        resolve([]);
      }
    });
  });
}


// 修改 deactivate 函数：
export function deactivate(): Thenable<void> | undefined {
  if (interfaceDecorator) {
    interfaceDecorator.dispose();
  }
  if (client) {
    return client.stop();
  }
  return undefined;
}

