$ProjectRoot = "C:\Users\TimeCraker\Desktop\game-backend-demo"
$DocsDir = Join-Path -Path $ProjectRoot -ChildPath "docs"
$WikiDir = Join-Path -Path $ProjectRoot -ChildPath ".wiki.git"
# 定义 GitHub Wiki 仓库的完整地址
$WikiRepoUrl = "https://github.com/TimeCraker/game-backend-demo.wiki.git"

if (-not (Test-Path -Path $DocsDir)) {
    New-Item -ItemType Directory -Path $DocsDir | Out-Null
    Write-Host "Success: Created output directory." -ForegroundColor Green
}

Write-Host "Starting document generation task..." -ForegroundColor Cyan

# 1. Project Tree
$TreeOutputPath = Join-Path -Path $DocsDir -ChildPath "project_tree.txt"
Write-Host "-> [1/3] Generating Project Tree..."
Push-Location -Path $ProjectRoot
cmd /c "tree /f /a" | Out-File -FilePath $TreeOutputPath -Encoding utf8
Pop-Location

# 2. Merge Code
$CodeOutputPath = Join-Path -Path $DocsDir -ChildPath "all_code_merged.txt"
Write-Host "-> [2/3] Merging Go source files..."
Get-ChildItem -Path $ProjectRoot -Filter *.go -Recurse | 
    Where-Object { $_.FullName -notmatch "vendor" } | 
    ForEach-Object {
        $content = Get-Content $_.FullName -Encoding UTF8
        "`n--- FILE: $($_.FullName) ---`n"
        $content
    } | Out-File -FilePath $CodeOutputPath -Encoding UTF8

# ===== 新增代码 START =====
# 3. Sync GitHub Wiki
Write-Host "-> [3/3] Syncing and Merging Wiki notes..."
if (-not (Test-Path -Path $WikiDir)) {
    Write-Host "   Wiki directory not found. Cloning from GitHub..." -ForegroundColor Yellow
    Push-Location -Path $ProjectRoot
    # 如果不存在，则克隆仓库并命名为 .wiki.git
    git clone $WikiRepoUrl ".wiki.git"
    Pop-Location
} else {
    Write-Host "   Wiki directory exists. Pulling latest changes..." -ForegroundColor Yellow
    Push-Location -Path $WikiDir
    # 如果已存在，则进入目录拉取最新内容
    git pull
    Pop-Location
}
# ===== 新增代码 END =====

# 4. Merge Wiki
$WikiOutputPath = Join-Path -Path $DocsDir -ChildPath "all_wiki_merged.txt"
if (Test-Path -Path $WikiDir) {
    Get-ChildItem -Path $WikiDir -Filter *.md -Recurse | 
        ForEach-Object {
            $header = "`n--- WIKI PAGE: $($_.Name) ---`n"
            Write-Output $header
            Get-Content $_.FullName -Encoding UTF8
        } | Out-File -FilePath $WikiOutputPath -Encoding UTF8
    Write-Host "   Wiki notes successfully merged." -ForegroundColor Green
} else {
    # 防止网络问题导致 clone 失败后的容错处理
    Write-Host "   Warning: Failed to clone Wiki directory, skipping merge." -ForegroundColor Red
}

Write-Host "DONE! All files saved to $DocsDir" -ForegroundColor Green