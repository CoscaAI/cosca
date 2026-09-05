@echo off
setlocal EnableExtensions
title COSCA - Environment Scanner

echo ==========================================
echo        COSCA ENVIRONMENT SCANNER
echo ==========================================
echo.

echo [SYSTEM]
ver
echo.
systeminfo | findstr /B /C:"OS Name" /C:"OS Version" /C:"System Type"
echo.

echo [PATH]
where git 2>nul
where go 2>nul
where cmake 2>nul
where ninja 2>nul
where cl 2>nul
where clang 2>nul
where clang-cl 2>nul
where node 2>nul
where npm 2>nul
where python 2>nul
where docker 2>nul
where dotnet 2>nul
echo.

echo [VERSIONS]

echo --- Git ---
git --version 2>&1

echo --- Go ---
go version 2>&1

echo --- CMake ---
cmake --version 2>&1

echo --- Ninja ---
ninja --version 2>&1

echo --- Node ---
node --version 2>&1

echo --- NPM ---
npm --version 2>&1

echo --- Python ---
python --version 2>&1

echo --- .NET ---
dotnet --info 2>&1

echo --- Docker ---
docker --version 2>&1

echo.
echo [C/C++ TOOLCHAIN]

where cl >nul 2>&1
if errorlevel 1 (
    echo [FAIL] MSVC compiler: cl.exe NOT FOUND
) else (
    echo [OK] MSVC compiler found
)

where link >nul 2>&1
if errorlevel 1 (
    echo [FAIL] MSVC linker: link.exe NOT FOUND
) else (
    echo [OK] MSVC linker found
)

where rc >nul 2>&1
if errorlevel 1 (
    echo [FAIL] Windows SDK resource compiler: rc.exe NOT FOUND
) else (
    echo [OK] Windows SDK resource compiler found
)

where mt >nul 2>&1
if errorlevel 1 (
    echo [FAIL] Windows SDK manifest tool: mt.exe NOT FOUND
) else (
    echo [OK] Windows SDK manifest tool found
)

echo.
echo [VISUAL STUDIO]

where vswhere >nul 2>&1
if errorlevel 1 (
    echo [WARN] vswhere.exe not found in PATH
) else (
    vswhere -latest -products * -property installationPath
)

echo.
echo [GO ENVIRONMENT]

go env GOPATH 2>&1
go env GOROOT 2>&1
go env GOOS 2>&1
go env GOARCH 2>&1
go env CGO_ENABLED 2>&1

echo.
echo [NODE]

npm config get prefix 2>&1

echo.
echo [GIT]

git config --global --get user.name 2>&1
git config --global --get user.email 2>&1

echo.
echo [DISK]

wmic logicaldisk get DeviceID,FreeSpace,Size 2>nul

echo.
echo ==========================================
echo SCAN COMPLETE
echo ==========================================
pause