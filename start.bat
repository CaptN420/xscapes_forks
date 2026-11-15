@echo off
rem start.bat - build xscapes if needed, then start what you pick.
rem   start.bat            menu
rem   start.bat hermes     run Hermes Agent inside the scape (also: claude, kimi, shell, scape)
rem   start.bat site       serve the site/manual at http://127.0.0.1:8000
rem   start.bat install    build + write the hooks for every agent found
setlocal EnableExtensions
cd /d "%~dp0"

set "EXE=%~dp0xscapes.exe"

rem --- build (go's cache makes an unchanged tree instant); fall back to an existing exe
where go >nul 2>nul || (if exist "%EXE%" (goto :built) else goto :nogo)
:build
where go >nul 2>nul || goto :nogo
echo [xscapes] building...
go build -o "%EXE%" .
if errorlevel 1 (
  if exist "%EXE%" (
    echo [xscapes] build failed; using the bundled Windows executable.
    goto :built
  )
  echo [xscapes] build failed and no bundled executable is available.
  pause
  exit /b 1
)
:built

if not "%~1"=="" (set "CHOICE=%~1" & goto :dispatch)

:menu
echo.
echo   xscapes
echo   -------
echo   1  Hermes Agent   inside the scape
echo   2  Claude Code    inside the scape
echo   3  Kimi Code CLI  inside the scape
echo   4  a shell        inside the scape
echo   5  the scape alone (xscapes -live)
echo   6  install hooks  (claude, kimi, hermes)
echo   7  serve the site/manual on http://127.0.0.1:8000
echo   8  version
echo   q  quit
set /p CHOICE=  pick: 
if "%CHOICE%"=="1" set CHOICE=hermes
if "%CHOICE%"=="2" set CHOICE=claude
if "%CHOICE%"=="3" set CHOICE=kimi
if "%CHOICE%"=="4" set CHOICE=shell
if "%CHOICE%"=="5" set CHOICE=scape
if "%CHOICE%"=="6" set CHOICE=install
if "%CHOICE%"=="7" set CHOICE=site
if "%CHOICE%"=="8" set CHOICE=version

:dispatch
if /i "%CHOICE%"=="hermes"  ("%EXE%" hermes & goto :end)
if /i "%CHOICE%"=="claude"  ("%EXE%" claude & goto :end)
if /i "%CHOICE%"=="kimi"    ("%EXE%" kimi & goto :end)
if /i "%CHOICE%"=="shell"   ("%EXE%" inside cmd & goto :end)
if /i "%CHOICE%"=="scape"   ("%EXE%" -live & goto :end)
if /i "%CHOICE%"=="version" ("%EXE%" version & goto :end)
if /i "%CHOICE%"=="install" goto :install
if /i "%CHOICE%"=="site"    goto :site
if /i "%CHOICE%"=="q" goto :end
echo unknown choice: %CHOICE%
goto :menu

:install
for %%a in (claude kimi hermes) do (
  echo.
  echo === %%a ===
  "%EXE%" install %%a --apply
)
goto :end

:site
where python >nul 2>nul && (start "" http://127.0.0.1:8000/manual.html & python -m http.server 8000 --bind 127.0.0.1 --directory site & goto :end)
where py >nul 2>nul && (start "" http://127.0.0.1:8000/manual.html & py -m http.server 8000 --bind 127.0.0.1 --directory site & goto :end)
echo Python was not found; open site\manual.html directly in a browser.
goto :end

:nogo
echo Go is not installed and xscapes.exe is missing. Install Go 1.27+ (winget install GoLang.Go) or download a release binary.
pause
exit /b 1

:end
endlocal
