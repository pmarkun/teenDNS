package setup

import "errors"

// Windows install and removal scripts. Both self-elevate via UAC and configure
// DNS-over-TLS on Windows 11 24H2+ (build >= 26100) using the netsh dnsclient
// commands. Older Windows builds cannot script DoT, so the installer aborts
// with a clear pointer to the manual configuration in the panel — it never
// silently falls back to plaintext DNS.

const windowsInstallTemplate = `@echo off
setlocal EnableExtensions EnableDelayedExpansion
title teenDNS
goto :main

:main
net session >nul 2>&1
if %errorlevel% neq 0 goto :elevate

set "TEENDNS_HOST={{.Hostname}}"
set "TEENDNS_IP={{.IP}}"
set "TEENDNS_PORT={{.Port}}"
set "TEENDNS_TEST_DOMAIN={{.TestDomain}}"

if "%TEENDNS_IP%"=="" goto :noip

for /f "tokens=2 delims= " %%b in ('reg query "HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion" /v CurrentBuild 2^>nul') do set "TEENDNS_BUILD=%%b"
if not defined TEENDNS_BUILD goto :oldwindows
set /a TEENDNS_BUILD_INT=TEENDNS_BUILD
if %TEENDNS_BUILD_INT% lss 26100 goto :oldwindows

echo.
echo teenDNS
echo Configurando DNS seguro para %TEENDNS_HOST% na porta %TEENDNS_PORT% ...
echo.

set "TEENDNS_FAILED=0"
set "TEENDNS_INTERFACES=0"
echo Localizando redes ativas...
for /f "delims=" %%a in ('powershell -NoProfile -Command "(Get-NetAdapter -Physical | Where-Object Status -eq Up).Name"') do (
  set /a TEENDNS_INTERFACES+=1
  echo Configurando "%%a" ...
  netsh interface ipv4 set dns name="%%a" static %TEENDNS_IP% validate=no >nul 2>&1
  if !errorlevel! neq 0 (
    echo [!] Nao consegui configurar a rede "%%a".
    set "TEENDNS_FAILED=1"
  ) else (
    echo [OK] Servidor DNS de "%%a" aponta para o teenDNS
  )
)
if %TEENDNS_INTERFACES% equ 0 (
  echo [!] Nenhuma rede ativa encontrada.
  set "TEENDNS_FAILED=1"
)
if "%TEENDNS_FAILED%" neq "0" goto :failed

echo Ativando DNS-over-TLS...
netsh dns add global dot=yes >nul 2>&1
netsh dns add encryption server=%TEENDNS_IP% dothost=%TEENDNS_HOST%:%TEENDNS_PORT% autoupgrade=yes udpfallback=no >nul 2>&1
if errorlevel 1 goto :failed

reg add "HKLM\SOFTWARE\Policies\Microsoft\Edge" /v BuiltInDnsClientEnabled /t REG_DWORD /d 0 /f >nul 2>&1

ipconfig /flushdns >nul

echo Testando resolucao de %TEENDNS_TEST_DOMAIN% ...
powershell -NoProfile -Command "$ok=$false; for($i=0;$i -lt 6 -and -not $ok;$i++){ try { Resolve-DnsName -Name '%TEENDNS_TEST_DOMAIN%' -Type A -ErrorAction Stop | Out-Null; $ok=$true } catch { Start-Sleep -Milliseconds 800 } }; if($ok){exit 0}else{exit 1}"
if errorlevel 1 goto :testfailed

echo.
echo [OK] Servidor DNS configurado
echo [OK] DNS-over-TLS ativado
echo [OK] Perfil teenDNS conectado
echo [OK] Teste realizado com sucesso
echo.
echo O teenDNS esta funcionando neste computador.
echo Pressione qualquer tecla para fechar.
pause >nul
exit /b 0

:testfailed
echo.
echo [!] O teste de resolucao falhou.
echo Confira a rede e tente novamente, ou veja a configuracao manual
echo no painel teenDNS (Configuracao avancada).
echo.
echo Pressione qualquer tecla para fechar.
pause >nul
exit /b 1

:failed
echo.
echo Nao foi possivel concluir a configuracao.
echo.
echo Etapa que falhou:
echo Configuracao do DNS-over-TLS
echo.
echo Voce pode tentar novamente ou consultar a configuracao manual
echo no painel teenDNS.
echo.
echo Pressione qualquer tecla para fechar.
pause >nul
exit /b 1

:oldwindows
echo.
echo teenDNS
echo Esta versao do Windows nao suporta DNS-over-TLS por script.
echo E preciso o Windows 11 24H2 ou mais recente.
echo.
echo Configure manualmente pelo painel teenDNS:
echo   Endereco: %TEENDNS_HOST%
echo   IP:       %TEENDNS_IP%
echo   Porta:    %TEENDNS_PORT%
echo.
echo Pressione qualquer tecla para fechar.
pause >nul
exit /b 1

:noip
echo.
echo teenDNS
echo Ainda nao temos o IP publico do servidor teenDNS neste servidor.
echo Use a configuracao manual no painel (Configuracao avancada).
echo.
echo Pressione qualquer tecla para fechar.
pause >nul
exit /b 1

:elevate
echo Solicitando permissao de administrador...
powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
exit /b
`

const windowsRemoveTemplate = `@echo off
setlocal EnableExtensions
title teenDNS
goto :main

:main
net session >nul 2>&1
if %errorlevel% neq 0 goto :elevate

set "TEENDNS_IP={{.IP}}"

echo.
echo teenDNS
echo Removendo a configuracao teenDNS deste computador...
echo.

set "TEENDNS_INTERFACES=0"
for /f "delims=" %%a in ('powershell -NoProfile -Command "(Get-NetAdapter -Physical | Where-Object Status -eq Up).Name"') do (
  set /a TEENDNS_INTERFACES+=1
  netsh interface ipv4 set dns name="%%a" source=dhcp >nul 2>&1
  if !errorlevel! neq 0 (
    echo [!] Nao consegui restaurar "%%a"
  ) else (
    echo [OK] "%%a" voltou ao DNS automatico
  )
)
if %TEENDNS_INTERFACES% equ 0 echo [!] Nenhuma rede ativa encontrada.

if not "%TEENDNS_IP%"=="" netsh dns delete encryption server=%TEENDNS_IP% >nul 2>&1
netsh dns delete global >nul 2>&1
netsh dns set global dot=no >nul 2>&1

ipconfig /flushdns >nul

echo.
echo O DNS voltou a ser automatico (via roteador se a rede for configurada
echo assim). O teenDNS foi removido deste computador.
echo.
echo Pressione qualquer tecla para fechar.
pause >nul
exit /b 0

:elevate
echo Solicitando permissao de administrador...
powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
exit /b
`

// WindowsInstallBat returns a self-elevating script that points every active
// physical interface (Wi-Fi and Ethernet) at the profile's DoT resolver and
// enables DNS-over-TLS bound to the profile hostname.
func WindowsInstallBat(params Params) (string, error) {
	normalized, err := params.normalized()
	if err != nil {
		return "", err
	}
	if err := validatePort(normalized.Port); err != nil {
		return "", err
	}
	if normalized.IP == "" {
		return "", errors.New("resolver IP é obrigatório para o arquivo do Windows")
	}
	return windowsLineEndings(render(windowsInstallTemplate, normalized)), nil
}

// WindowsRemoveBat returns a script that returns the active interfaces to
// automatic (DHCP) DNS and removes the DoT encryption entry for the resolver.
func WindowsRemoveBat(params Params) (string, error) {
	normalized, err := params.normalized()
	if err != nil {
		return "", err
	}
	return windowsLineEndings(render(windowsRemoveTemplate, normalized)), nil
}