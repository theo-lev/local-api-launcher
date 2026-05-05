@echo off
setlocal

set ROOT=%~dp0
set OUT=%ROOT%dist
if not exist "%OUT%" mkdir "%OUT%"

echo =^> Building frontend...
cd /d "%ROOT%frontend"
call npm install
if errorlevel 1 goto error
call npm run build
if errorlevel 1 goto error

echo =^> Cross-compiling backend...
cd /d "%ROOT%backend"

echo    darwin/amd64  -^> dist\api-manager-darwin-amd64
set GOOS=darwin& set GOARCH=amd64& go build -o "%OUT%\api-manager-darwin-amd64" .
if errorlevel 1 goto error

echo    darwin/arm64  -^> dist\api-manager-darwin-arm64
set GOOS=darwin& set GOARCH=arm64& go build -o "%OUT%\api-manager-darwin-arm64" .
if errorlevel 1 goto error

echo    linux/amd64   -^> dist\api-manager-linux-amd64
set GOOS=linux& set GOARCH=amd64& go build -o "%OUT%\api-manager-linux-amd64" .
if errorlevel 1 goto error

echo    linux/arm64   -^> dist\api-manager-linux-arm64
set GOOS=linux& set GOARCH=arm64& go build -o "%OUT%\api-manager-linux-arm64" .
if errorlevel 1 goto error

echo    windows/amd64 -^> dist\api-manager-windows-amd64.exe
set GOOS=windows& set GOARCH=amd64& go build -o "%OUT%\api-manager-windows-amd64.exe" .
if errorlevel 1 goto error

echo.
echo Done! Binaries are in dist\
echo   macOS (Intel):   dist\api-manager-darwin-amd64
echo   macOS (Apple):   dist\api-manager-darwin-arm64
echo   Linux (x64):     dist\api-manager-linux-amd64
echo   Linux (ARM64):   dist\api-manager-linux-arm64
echo   Windows (x64):   dist\api-manager-windows-amd64.exe
echo.
echo Run the binary and open http://localhost:8080
goto end

:error
echo Build failed!
exit /b 1

:end
