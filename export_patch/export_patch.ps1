# 确保中文正常显示
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

# 【坑1】必须切换到脚本所在目录
# 否则 git 命令会在当前工作目录执行，找不到正确的仓库
Set-Location $PSScriptRoot

# 【坑2】禁止邮件头编码
# true: Subject 会变成 =?UTF-8?q?=E5=8D=87=E7=BA=A7... 这种转义格式
# false: Subject 保持原始中文，如 "升级现代化代码"
git config format.encodeEmailHeaders false

# 【坑3】首次运行时 origin/main 不存在会报错
# git rev-list --reverse origin/main..HEAD 需要 origin/main 的本地引用
# 如果从未 fetch 过，origin/main 是 unknown revision
if (-not (git rev-parse --verify origin/main 2>$null)) {
    git fetch origin
}

# 创建 patches 目录（如果不存在）
if (-not (Test-Path "patches")) {
    New-Item -ItemType Directory -Path "patches" -Force | Out-Null
}

# 清空旧补丁
Remove-Item patches/*.patch -ErrorAction SilentlyContinue

# 导出补丁（带中文文件名）
$i = 1
foreach ($hash in (git rev-list --reverse origin/main..HEAD)) {
    $msg = git log -1 --format="%s" $hash
    $safe = $msg -replace '[\\/:*?"<>|]', '-'
    if ($safe.Length -gt 50) { $safe = $safe.Substring(0, 50) }
    $num = "{0:D4}" -f $i
    
    # 【坑4】绝不能用 PowerShell 的 > 重定向！
    # 错误写法: git format-patch -1 $hash --stdout > "patches/$num-$safe.patch"
    # PowerShell 的 > 默认用 UTF-16 编码，会破坏补丁格式
    # Goland/IDE 会识别为非补丁文件
    #
    # 正确做法: 让 git 直接写入文件 (-o patches)
    # git 原生输出就是正确的 UTF-8 邮件格式
    $tmp = git format-patch -1 $hash -o patches
    $newName = "$num-$safe.patch"
    Rename-Item -Path $tmp -NewName $newName -Force
    
    Write-Host "patches/$num-$safe.patch"
    $i++
}
