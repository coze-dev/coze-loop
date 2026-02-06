#!/usr/bin/env node

const path = require('path');
const fs = require('fs');
const { execSync } = require('child_process');

class ExportTransformer {
  constructor() {
    this.modifiedFiles = [];
    this.originalContents = new Map();
  }

  // 查找所有包含 "export * as" 的 TypeScript 文件
  findFilesWithExportStarAs(dir) {
    const files = [];

    function scanDir(currentDir) {
      const entries = fs.readdirSync(currentDir, { withFileTypes: true });

      for (const entry of entries) {
        const fullPath = path.join(currentDir, entry.name);

        if (
          entry.isDirectory() &&
          entry.name !== 'node_modules' &&
          entry.name !== 'dist'
        ) {
          scanDir(fullPath);
        } else if (entry.isFile() && entry.name.endsWith('.ts')) {
          const content = fs.readFileSync(fullPath, 'utf-8');
          if (content.includes('export * as ')) {
            files.push(fullPath);
          }
        }
      }
    }

    scanDir(dir);
    return files;
  }

  // 转换 "export * as xxx" 为 "import * as xxx" + "export { xxx }"
  transformExportStarAs(content) {
    const lines = content.split('\n');
    const transformedLines = [];
    const exportNames = [];

    for (const line of lines) {
      const trimmedLine = line.trim();

      // 匹配 export * as xxx from 'yyy'
      const match = trimmedLine.match(
        /^export\s+\*\s+as\s+(\w+)\s+from\s+['"]([^'"]+)['"];?$/,
      );

      if (match) {
        const [, aliasName, modulePath] = match;
        // 转换为 import * as xxx from 'yyy'
        transformedLines.push(`import * as ${aliasName} from '${modulePath}';`);
        exportNames.push(aliasName);
      } else {
        transformedLines.push(line);
      }
    }

    // 如果有转换，添加 export { xxx } 声明
    if (exportNames.length > 0) {
      transformedLines.push('');
      transformedLines.push(`export { ${exportNames.join(', ')} };`);
    }

    return transformedLines.join('\n');
  }

  // 备份并转换文件
  transformFiles() {
    const srcDir = path.join(__dirname, 'src');
    const files = this.findFilesWithExportStarAs(srcDir);

    console.log(`找到 ${files.length} 个需要转换的文件:`);

    for (const filePath of files) {
      console.log(`- ${path.relative(__dirname, filePath)}`);

      // 备份原始内容
      const originalContent = fs.readFileSync(filePath, 'utf-8');
      this.originalContents.set(filePath, originalContent);

      // 转换内容
      const transformedContent = this.transformExportStarAs(originalContent);

      // 写入转换后的内容
      fs.writeFileSync(filePath, transformedContent, 'utf-8');
      this.modifiedFiles.push(filePath);
    }

    console.log(`已转换 ${this.modifiedFiles.length} 个文件\n`);
  }

  // 恢复所有修改的文件
  restoreFiles() {
    console.log('\n开始恢复文件...');

    for (const filePath of this.modifiedFiles) {
      const originalContent = this.originalContents.get(filePath);
      if (originalContent) {
        fs.writeFileSync(filePath, originalContent, 'utf-8');
        console.log(`已恢复: ${path.relative(__dirname, filePath)}`);
      }
    }

    console.log(`已恢复 ${this.modifiedFiles.length} 个文件`);
  }

  // 使用 git 恢复文件 (备用方案)
  restoreWithGit() {
    console.log('\n使用 git 恢复文件...');

    try {
      // 检查是否有暂存的更改
      const status = execSync('git status --porcelain', { encoding: 'utf-8' });

      if (status.trim()) {
        // 恢复所有更改
        execSync('git checkout -- .', { stdio: 'inherit' });
        console.log('已使用 git 恢复所有文件');
      } else {
        console.log('没有需要恢复的文件');
      }
    } catch (error) {
      console.error('Git 恢复失败:', error.message);
      throw error;
    }
  }

  // 执行构建
  async build() {
    console.log('开始执行构建...\n');

    try {
      // 1. 转换文件
      this.transformFiles();

      // 2. 执行构建
      console.log('运行 rslib build...');
      execSync('npx rslib build', {
        stdio: 'inherit',
        cwd: __dirname,
      });
      console.log('\n✅ 构建成功完成!');
    } catch (error) {
      console.error('\n❌ 构建失败:', error.message);
      throw error;
    } finally {
      // 3. 恢复文件
      try {
        this.restoreFiles();
        console.log('\n✅ 文件恢复完成!');
      } catch {
        console.error('\n⚠️  文件恢复失败，尝试使用 git 恢复...');
        try {
          this.restoreWithGit();
          console.log('\n✅ Git 恢复完成!');
        } catch (gitError) {
          console.error('\n❌ Git 恢复也失败了:', gitError.message);
          console.log('\n请手动恢复以下文件:');
          this.modifiedFiles.forEach(file => {
            console.log(`  - ${path.relative(__dirname, file)}`);
          });
        }
      }
    }
  }
}

// 主执行函数
async function main() {
  const transformer = new ExportTransformer();

  try {
    await transformer.build();
    console.log('\n🎉 构建流程完成!');
    process.exit(0);
  } catch (error) {
    console.error('\n💥 构建流程失败:', error.message);
    process.exit(1);
  }
}

// 处理中断信号
process.on('SIGINT', () => {
  console.log('\n\n⚠️  收到中断信号，正在清理...');
  const transformer = new ExportTransformer();
  try {
    transformer.restoreWithGit();
  } catch (error) {
    console.error('清理失败:', error.message);
  }
  process.exit(1);
});

// 运行主函数
if (require.main === module) {
  main();
}

module.exports = ExportTransformer;
