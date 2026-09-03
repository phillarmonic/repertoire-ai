; Repertoire NSIS installer — per-user, no elevation.
;
; Installs entirely under the current user's profile:
;
;   %LOCALAPPDATA%\Programs\Repertoire\
;       repertoire.exe
;       uninstall.exe
;
; adds that directory to the CURRENT USER's PATH (HKCU\Environment) so
; `repertoire` is available from new terminal sessions, and registers an
; Add/Remove Programs entry under HKCU. No administrator privileges are
; required and nothing under Program Files is touched.
;
; Build (from the repo root, after cross-compiling the Windows binary):
;
;   makensis -DVERSION=0.1.0 installer\repertoire.nsi
;
; Optional defines:
;   -DVERSION=1.2.3       version stamped into the registry and the file name
;   -DBINARY_AMD64=path   x64 Windows binary
;                         (default: dist\repertoire-windows-amd64.exe)
;   -DBINARY_ARM64=path   ARM64 Windows binary
;                         (default: dist\repertoire-windows-arm64.exe)
;
; Both binaries are packaged; the installer extracts only the one matching the
; machine's native architecture (AMD64 or ARM64).

!ifndef VERSION
  !define VERSION "0.1.0"
!endif
!ifndef BINARY_AMD64
  !define BINARY_AMD64 "..\dist\repertoire-windows-amd64.exe"
!endif
!ifndef BINARY_ARM64
  !define BINARY_ARM64 "..\dist\repertoire-windows-arm64.exe"
!endif

!define APP_NAME   "Repertoire"
!define APP_SLUG   "repertoire"
!define PUBLISHER  "Phillarmonic Software"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_SLUG}"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "WinMessages.nsh"

Name "${APP_NAME}"
OutFile "..\dist\repertoire-setup-${VERSION}.exe"
Unicode True
InstallDir "$LOCALAPPDATA\Programs\Repertoire"
InstallDirRegKey HKCU "Software\${APP_SLUG}" "InstallDir"
RequestExecutionLevel user

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

;----------------------------------------------------------------
; Helpers
;----------------------------------------------------------------

; AddToUserPath ensures $0 is present in the current user's PATH as its own
; segment. Any existing segment equal to $0 is removed first, so reinstalls
; never duplicate the entry.
Function AddToUserPath
  Push $1 ; current PATH
  Push $2 ; rebuilt PATH
  Push $3 ; current segment
  Push $4 ; cursor
  Push $5 ; PATH length
  Push $6 ; scratch char
  Push $7 ; scratch

  ReadRegStr $1 HKCU "Environment" "Path"
  StrCpy $2 ""
  StrLen $5 $1
  StrCpy $4 0

  ${DoWhile} $4 < $5
    StrCpy $3 ""
    ${Do}
      StrCpy $6 $1 1 $4
      ${If} $6 == ";"
        IntOp $4 $4 + 1
        ${Break}
      ${EndIf}
      StrCpy $3 "$3$6"
      IntOp $4 $4 + 1
      ${If} $4 >= $5
        ${Break}
      ${EndIf}
    ${Loop}

    ${If} $3 != $0
      ${If} $2 == ""
        StrCpy $2 "$3"
      ${Else}
        StrCpy $2 "$2;$3"
      ${EndIf}
    ${EndIf}
  ${Loop}

  ; Append the install directory.
  ${If} $2 == ""
    StrCpy $2 "$0"
  ${Else}
    StrCpy $2 "$2;$0"
  ${EndIf}
  WriteRegExpandStr HKCU "Environment" "Path" "$2"

  Pop $7
  Pop $6
  Pop $5
  Pop $4
  Pop $3
  Pop $2
  Pop $1
FunctionEnd

;----------------------------------------------------------------
; Installer
;----------------------------------------------------------------

Section "Install"
  SetOutPath "$INSTDIR"

  ; Detect the machine's native architecture. NSIS runs as a 32-bit process, so
  ; PROCESSOR_ARCHITECTURE in the process environment is unreliable under x86
  ; emulation; the machine-level value in HKLM reports the true architecture
  ; (AMD64 or ARM64) regardless of the installer's bitness.
  ReadRegStr $R0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "PROCESSOR_ARCHITECTURE"
  ${If} $R0 == "ARM64"
    DetailPrint "Installing ARM64 build of ${APP_NAME}."
    File /oname=repertoire.exe "${BINARY_ARM64}"
  ${Else}
    DetailPrint "Installing x64 (AMD64) build of ${APP_NAME}."
    File /oname=repertoire.exe "${BINARY_AMD64}"
  ${EndIf}

  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Add/Remove Programs entry (per-user) and the remembered install dir.
  WriteRegStr HKCU "Software\${APP_SLUG}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\repertoire.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1

  ; Add the install dir to the current user's PATH so `repertoire` resolves
  ; from new terminals.
  StrCpy $0 "$INSTDIR"
  Call AddToUserPath
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=1000
SectionEnd

;----------------------------------------------------------------
; Uninstaller
;----------------------------------------------------------------

Function un.RemoveFromUserPath
  Push $1
  Push $2
  Push $3
  Push $4
  Push $5
  Push $6
  Push $7

  ReadRegStr $1 HKCU "Environment" "Path"
  StrCpy $2 ""
  StrLen $5 $1
  StrCpy $4 0

  ${DoWhile} $4 < $5
    StrCpy $3 ""
    ${Do}
      StrCpy $6 $1 1 $4
      ${If} $6 == ";"
        IntOp $4 $4 + 1
        ${Break}
      ${EndIf}
      StrCpy $3 "$3$6"
      IntOp $4 $4 + 1
      ${If} $4 >= $5
        ${Break}
      ${EndIf}
    ${Loop}

    ${If} $3 != $0
      ${If} $2 == ""
        StrCpy $2 "$3"
      ${Else}
        StrCpy $2 "$2;$3"
      ${EndIf}
    ${EndIf}
  ${Loop}

  ${If} $2 == ""
    DeleteRegValue HKCU "Environment" "Path"
  ${Else}
    WriteRegExpandStr HKCU "Environment" "Path" "$2"
  ${EndIf}

  Pop $7
  Pop $6
  Pop $5
  Pop $4
  Pop $3
  Pop $2
  Pop $1
FunctionEnd

Section "Uninstall"
  ; Remove the exact install dir from the current user's PATH.
  StrCpy $0 "$INSTDIR"
  Call un.RemoveFromUserPath
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=1000

  Delete "$INSTDIR\repertoire.exe"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\${APP_SLUG}"
SectionEnd
