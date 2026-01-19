const { performance } = require('perf_hooks');
const path = require('path');
const fs = require('fs');
const { execSync } = require('child_process');
const isBeta = process.argv.includes('--beta');

const pkgPath = path.resolve(__dirname, '..', 'package.json');

// 1. 备份 package.json
const backupPkg = JSON.parse(fs.readFileSync(pkgPath, 'utf-8'));

// 2. 读取 package.json
const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf-8'));

// 3. 获取线上最新版本
let latestVersion = '0.0.0';
let isFirstPublish = false;
try {
  const result = execSync(
    `npm view ${pkg.botPublishConfig.name} versions --json`,
    {
      encoding: 'utf-8',
    },
  );
  console.log('获取线上版本成功', result);
  const versions = JSON.parse(result);
  latestVersion = versions.findLast(v => !v.includes('-beta'));
} catch (e) {
  console.error('获取线上版本失败:', e);
  console.warn(
    '获取线上版本失败，可能是首次发布或网络问题，使用 0.0.0 作为基准',
  );
  isFirstPublish = true;
}

// 4. 计算新版本号（patch +1）
function bumpPatch(version) {
  if (isBeta) {
    console.warn('检测到 --beta 参数，版本号将不进行自动递增');
    return `${version}-beta-${performance.now()}`; // 如果是 beta 版本，不递增版本号
  } else {
    console.log('检测到非 beta 发布，自动递增 patch 版本号');
    const parts = version.split('.').map(Number);
    if (parts.length !== 3 || parts.some(isNaN)) {
      return '0.0.1';
    }

    parts[2] += 1;
    return parts.join('.');
  }
}
const newVersion = bumpPatch(latestVersion);
console.log('即将发布的新版本:', newVersion);

// 5. 处理发布配置
const botPublishConfig = pkg.botPublishConfig || {};

// 6. 替换 workspace:* 依赖为 latest
function replaceWorkspaceDeps(deps) {
  if (!deps) {
    return deps;
  }

  const newDeps = {};
  for (const [pkgName, version] of Object.entries(deps)) {
    if (version === 'workspace:*') {
      newDeps[pkgName] = 'latest';
      console.log(`替换依赖 ${pkgName}: workspace:* -> latest`);
    } else {
      newDeps[pkgName] = version;
    }
  }
  return newDeps;
}

// 替换所有依赖类型中的 workspace:*
const newPkg = { ...pkg, ...botPublishConfig, version: newVersion };
newPkg.dependencies = replaceWorkspaceDeps(newPkg.dependencies);

delete newPkg.botPublishConfig;

// 7. 写入临时 package.json
fs.writeFileSync(pkgPath, JSON.stringify(newPkg, null, 2));

// 8. 执行 build 和 publish
try {
  execSync('npm run build-sdk', {
    stdio: 'inherit',
  });

  // 删除 dist 目录下的 tsconfig.build 文件
  const distPath = path.resolve(__dirname, '..', 'dist');
  const tsconfigFiles = ['tsconfig.build.json', 'tsconfig.build.tsbuildinfo'];

  tsconfigFiles.forEach(file => {
    const filePath = path.join(distPath, 'types', file);
    if (fs.existsSync(filePath)) {
      fs.unlinkSync(filePath);
      console.log(`已删除 ${filePath}`);
    }
  });

  // start_aigc
  // 将 dist 文件夹下的 .mjs 文件重命名为 .js 文件
  function renameMjsToJs(dir) {
    const files = fs.readdirSync(dir);
    files.forEach(file => {
      const filePath = path.join(dir, file);
      const stat = fs.statSync(filePath);

      if (stat.isDirectory()) {
        renameMjsToJs(filePath); // 递归处理子目录
      } else if (file.endsWith('.mjs')) {
        const newFilePath = filePath.replace(/\.mjs$/, '.js');
        fs.renameSync(filePath, newFilePath);
        console.log(`已重命名: ${filePath} -> ${newFilePath}`);
      }
    });
  }

  if (fs.existsSync(distPath)) {
    renameMjsToJs(distPath);
  }
  // end_aigc

  let publishCommand = isFirstPublish
    ? 'npm publish --access public'
    : 'npm publish';

  if (isBeta) {
    publishCommand += ' --tag beta';
  }
  console.log(`执行发布命令: ${publishCommand}`);
  execSync(publishCommand, { stdio: 'inherit' });
} catch (e) {
  console.error('发布失败:', e);
} finally {
  // 9. 还原 package.json
  try {
    fs.writeFileSync(pkgPath, JSON.stringify(backupPkg, null, 2));
    console.log('package.json 已还原');
  } catch (restoreError) {
    console.error('还原 package.json 失败:', restoreError);
    // 如果备份文件不存在，尝试从 git 恢复
    try {
      execSync('git checkout -- package.json', { stdio: 'inherit' });
      console.log('已从 git 恢复 package.json');
    } catch (gitError) {
      console.error('从 git 恢复也失败:', gitError);
    }
  }
}
