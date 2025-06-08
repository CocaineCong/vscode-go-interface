import {
  createConnection,
  TextDocuments,
  ProposedFeatures,
  InitializeParams,
  DidChangeConfigurationNotification,
  TextDocumentPositionParams,
  TextDocumentSyncKind,
  InitializeResult,
  CodeLens,
  CodeLensParams,
  Location,
} from 'vscode-languageserver/node';

import { TextDocument } from 'vscode-languageserver-textdocument';
import * as fs from 'fs';
import * as path from 'path';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

// 创建服务器连接
const connection = createConnection(ProposedFeatures.all);
const documents: TextDocuments<TextDocument> = new TextDocuments(TextDocument);

let hasConfigurationCapability = false;
let hasWorkspaceFolderCapability = false;
let hasDiagnosticRelatedInformationCapability = false;

connection.onInitialize((params: InitializeParams) => {
  const capabilities = params.capabilities;

  hasConfigurationCapability = !!(capabilities.workspace && !!capabilities.workspace.configuration);
  hasWorkspaceFolderCapability = !!(capabilities.workspace && !!capabilities.workspace.workspaceFolders);
  hasDiagnosticRelatedInformationCapability = !!(capabilities.textDocument &&
    capabilities.textDocument.publishDiagnostics &&
    capabilities.textDocument.publishDiagnostics.relatedInformation);

  const result: InitializeResult = {
    capabilities: {
      textDocumentSync: TextDocumentSyncKind.Incremental,
      completionProvider: {
        resolveProvider: true
      },
      codeLensProvider: {
        resolveProvider: true
      },
      definitionProvider: true,
      implementationProvider: true
    }
  };
  
  if (hasWorkspaceFolderCapability) {
    result.capabilities.workspace = {
      workspaceFolders: {
        supported: true
      }
    };
  }
  
  return result;
});

connection.onInitialized(() => {
  if (hasConfigurationCapability) {
    connection.client.register(DidChangeConfigurationNotification.type, undefined);
  }
  if (hasWorkspaceFolderCapability) {
    connection.workspace.onDidChangeWorkspaceFolders(_event => {
      connection.console.log('Workspace folder change event received.');
    });
  }
});

// 实现提供器 - 使用Go AST
connection.onImplementation(async (params: TextDocumentPositionParams) => {
  const document = documents.get(params.textDocument.uri);
  if (!document) {
    return [];
  }

  const position = params.position;
  const text = document.getText();
  const lines = text.split('\n');
  const currentLine = lines[position.line];
  
  // 获取当前位置的方法名
  const methodName = getMethodNameAtPosition(currentLine, position.character);
  if (!methodName) {
    return [];
  }

  // 使用Go AST分析器查找实现
  const implementations = await findMethodImplementationsWithAST(methodName, document.uri);
  return implementations;
});

// 定义提供器 - 使用Go AST
connection.onDefinition(async (params: TextDocumentPositionParams) => {
  const document = documents.get(params.textDocument.uri);
  if (!document) {
    return [];
  }

  const position = params.position;
  const text = document.getText();
  const lines = text.split('\n');
  const currentLine = lines[position.line];
  
  // 获取当前位置的方法名
  const methodName = getMethodNameAtPosition(currentLine, position.character);
  if (!methodName) {
    return [];
  }

  // 检查当前是否在方法实现中
  if (!isMethodImplementation(currentLine)) {
    return [];
  }

  // 使用Go AST分析器查找接口定义
  const interfaceDefinitions = await findInterfaceDefinitionsWithAST(methodName, document.uri);
  return interfaceDefinitions;
});

// 使用Go AST分析器查找方法实现
async function findMethodImplementationsWithAST(methodName: string, currentUri: string): Promise<Location[]> {
  const workspaceRoot = getWorkspaceRoot(currentUri);
  if (!workspaceRoot) {
    return [];
  }

  try {
    // 使用扩展目录中的分析器，而不是工作区目录
    const extensionPath = process.env.EXTENSION_PATH || __dirname;
    const analyzerPath = path.join(path.dirname(path.dirname(extensionPath)), 'ast-analyzer');
    
    // 确保分析器已编译
    await ensureAnalyzerBuilt(analyzerPath);
    
    // 执行Go分析器
    const { stdout } = await execAsync(`cd "${analyzerPath}" && go run main.go find-implementations "${workspaceRoot}" "${methodName}"`);
    
    const result = JSON.parse(stdout);
    const locations: Location[] = [];
    
    for (const impl of result.implementations || []) {
      locations.push({
        uri: `file://${impl.location.file}`,
        range: {
          start: { line: impl.location.line, character: impl.location.column },
          end: { line: impl.location.line, character: impl.location.column + methodName.length }
        }
      });
    }
    
    return locations;
  } catch (error) {
    connection.console.error(`Error finding implementations: ${error}`);
    return [];
  }
}

// 使用Go AST分析器查找接口定义
async function findInterfaceDefinitionsWithAST(methodName: string, currentUri: string): Promise<Location[]> {
  const workspaceRoot = getWorkspaceRoot(currentUri);
  if (!workspaceRoot) {
    return [];
  }

  try {
    // 使用扩展目录中的分析器，而不是工作区目录
    const extensionPath = process.env.EXTENSION_PATH || __dirname;
    const analyzerPath = path.join(path.dirname(path.dirname(extensionPath)), 'ast-analyzer');
    
    // 确保分析器已编译
    await ensureAnalyzerBuilt(analyzerPath);
    
    // 执行Go分析器
    const { stdout } = await execAsync(`cd "${analyzerPath}" && go run main.go find-interfaces "${workspaceRoot}" "${methodName}"`);
    
    const result = JSON.parse(stdout);
    const locations: Location[] = [];
    
    for (const intf of result.interfaces || []) {
      locations.push({
        uri: `file://${intf.location.file}`,
        range: {
          start: { line: intf.location.line, character: intf.location.column },
          end: { line: intf.location.line, character: intf.location.column + methodName.length }
        }
      });
    }
    
    return locations;
  } catch (error) {
    connection.console.error(`Error finding interfaces: ${error}`);
    return [];
  }
}

// 确保Go分析器已构建
async function ensureAnalyzerBuilt(analyzerPath: string): Promise<void> {
  try {
    // connection.console.log(`Checking analyzer path: ${analyzerPath}`);
    
    // 检查目录是否存在
    if (!fs.existsSync(analyzerPath)) {
      throw new Error(`AST analyzer directory not found: ${analyzerPath}`);
    }
    
    // 检查是否存在main.go
    const mainGoPath = path.join(analyzerPath, 'main.go');
    // connection.console.log(`Checking main.go at: ${mainGoPath}`);
    
    if (!fs.existsSync(mainGoPath)) {
      throw new Error(`AST analyzer main.go not found at: ${mainGoPath}`);
    }
    
    // connection.console.log('AST analyzer files found successfully');
    
    // 检查go.mod是否存在
    const goModPath = path.join(analyzerPath, 'go.mod');
    if (!fs.existsSync(goModPath)) {
      // connection.console.log('Initializing go module...');
      // 初始化go module
      await execAsync(`cd "${analyzerPath}" && go mod init ast-analyzer`);
    }
    
    // connection.console.log('AST analyzer is ready');
  } catch (error) {
    connection.console.error(`Error ensuring analyzer built: ${error}`);
    throw error;
  }
}

// 辅助函数：获取指定位置的方法名
function getMethodNameAtPosition(line: string, character: number): string | null {
  // 匹配接口方法定义
  const interfaceMethodMatch = line.match(/(\w+)\s*\([^)]*\)/);
  if (interfaceMethodMatch) {
    return interfaceMethodMatch[1];
  }
  
  // 匹配方法实现
  const methodImplMatch = line.match(/func\s*\([^)]+\)\s*(\w+)\s*\([^)]*\)/);
  if (methodImplMatch) {
    return methodImplMatch[1];
  }
  
  return null;
}

// 辅助函数：检查是否为方法实现
function isMethodImplementation(line: string): boolean {
  return /^func\s*\([^)]+\)\s*\w+\s*\([^)]*\)/.test(line.trim());
}

// 获取工作区根目录
function getWorkspaceRoot(uri: string): string | null {
  try {
    const filePath = uri.replace('file://', '');
    // connection.console.log(`Getting workspace root for file: ${filePath}`);
    
    let currentDir = path.dirname(filePath);
    // connection.console.log(`Starting directory: ${currentDir}`);
    
    // 向上查找go.mod文件
    while (currentDir !== path.dirname(currentDir)) {
      const goModPath = path.join(currentDir, 'go.mod');
      // connection.console.log(`Checking for go.mod at: ${goModPath}`);
      
      if (fs.existsSync(goModPath)) {
        // connection.console.log(`Found workspace root: ${currentDir}`);
        return currentDir;
      }
      currentDir = path.dirname(currentDir);
    }
    
    // 如果没找到go.mod，返回文件所在目录
    const fallbackDir = path.dirname(filePath);
    // connection.console.log(`No go.mod found, using fallback: ${fallbackDir}`);
    return fallbackDir;
  } catch (error) {
    connection.console.error(`Error getting workspace root: ${error}`);
    return null;
  }
}

// 递归查找Go文件
function findGoFiles(dir: string): string[] {
  const goFiles: string[] = [];
  
  try {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      
      if (entry.isDirectory()) {
        // 跳过vendor和.git目录
        if (entry.name !== 'vendor' && entry.name !== '.git' && !entry.name.startsWith('.')) {
          goFiles.push(...findGoFiles(fullPath));
        }
      } else if (entry.isFile() && entry.name.endsWith('.go')) {
        goFiles.push(fullPath);
      }
    }
  } catch (error) {
    // 忽略读取错误
  }
  
  return goFiles;
}

// CodeLens 提供器 - 使用 Go AST 分析器
connection.onCodeLens(async (params: CodeLensParams): Promise<CodeLens[]> => {
  const document = documents.get(params.textDocument.uri);
  if (!document) {
    // connection.console.log('CodeLens: No document found');
    return [];
  }

  const codeLenses: CodeLens[] = [];
  const workspaceRoot = getWorkspaceRoot(document.uri);
  if (!workspaceRoot) {
    connection.console.log('CodeLens: No workspace root found');
    return [];
  }

  // connection.console.log(`CodeLens: Processing file ${document.uri}`);
  // connection.console.log(`CodeLens: Workspace root ${workspaceRoot}`);

  try {
    // 使用扩展目录中的分析器，而不是工作区目录
    // 这里需要获取扩展的安装路径
    const extensionPath = process.env.EXTENSION_PATH || __dirname;
    const analyzerPath = path.join(path.dirname(path.dirname(extensionPath)), 'ast-analyzer');
    // connection.console.log(`CodeLens: Analyzer path ${analyzerPath}`);
    
    // 确保分析器已编译
    await ensureAnalyzerBuilt(analyzerPath);
    
    // 获取当前文件的所有接口方法
    const filePath = document.uri.replace('file://', '');
    // connection.console.log(`CodeLens: Analyzing file ${filePath}`);
    
    const interfaceCommand = `cd "${analyzerPath}" && go run main.go find-file-interfaces "${filePath}"`;
    // connection.console.log(`CodeLens: Running command: ${interfaceCommand}`);
    
    const { stdout: interfaceResult } = await execAsync(interfaceCommand);
    // connection.console.log(`CodeLens: Interface result: ${interfaceResult}`);
    
    const interfaces = JSON.parse(interfaceResult);
    // connection.console.log(`CodeLens: Parsed interfaces:${JSON.stringify(interfaces)}`);
    
    // 为每个接口方法添加 CodeLens
    for (const intf of interfaces.interfaces || []) {
      // connection.console.log(`CodeLens: Processing interface method ${intf.name}`);
      
      // 先查找该方法的实现数量
      const implCommand = `cd "${analyzerPath}" && go run main.go find-implementations "${workspaceRoot}" "${intf.name}"`;
      // connection.console.log(`CodeLens: Running impl command: ${implCommand}`);
      
      const { stdout: implResult } = await execAsync(implCommand);
      // connection.console.log(`CodeLens: Implementation result: ${implResult}`);
      
      const implementations = JSON.parse(implResult);
      const implCount = implementations.implementations?.length || 0;
      // connection.console.log(`CodeLens: Found ${implCount} implementations for ${intf.name}`);
      
      if (implCount > 0) {
        const codeLens = {
          range: {
            start: { line: intf.location.line, character: intf.location.column },
            end: { line: intf.location.line, character: intf.location.column + intf.name.length }
          },
          command: {
            title: `🔍 ${implCount} implementation${implCount > 1 ? 's' : ''}`,
            command: 'goInterfaceNavigator.findImplementations',
            arguments: [document.uri, intf.name, intf.location.line]
          }
        };
        connection.console.log(`CodeLens: Adding CodeLens:${JSON.stringify(codeLens)}`,);
        codeLenses.push(codeLens);
      }
    }
    
  } catch (error) {
    connection.console.error(`Error generating CodeLens: ${error}`);
  }

  // connection.console.log(`CodeLens: Returning ${codeLenses.length} code lenses`);
  return codeLenses;
});

documents.listen(connection);
connection.listen();