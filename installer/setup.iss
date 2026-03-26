; OpenSmurfManager Inno Setup Script
; Production-ready installer

#define MyAppName "OpenSmurfManager"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "OpenSmurfManager"
#define MyAppURL "https://github.com/ajanitshimanga/OpenSmurfManager"
#define MyAppExeName "OpenSmurfManager.exe"

[Setup]
; Basic app info
AppId={{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}

; Install location
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes

; Output settings
OutputDir=..\build\installer
OutputBaseFilename=OpenSmurfManager-Setup-{#MyAppVersion}
SetupIconFile=..\build\windows\icon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}

; Compression
Compression=lzma2/ultra64
SolidCompression=yes
LZMAUseSeparateProcess=yes

; Privileges (install for current user by default, no admin needed)
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

; Modern look
WizardStyle=modern
WizardSizePercent=100

; Minimum Windows version (Windows 10)
MinVersion=10.0

; Uninstall settings
UninstallDisplayName={#MyAppName}
CreateUninstallRegKey=yes

; Allow closing running app during install/update
CloseApplications=yes
CloseApplicationsFilter=*.exe
RestartApplications=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "startupicon"; Description: "Start with Windows"; GroupDescription: "Startup:"; Flags: unchecked

[Files]
; Main executable
Source: "..\build\bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

; WebView2 loader (if needed)
; Source: "..\build\bin\WebView2Loader.dll"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist

[Icons]
; Start Menu
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"

; Desktop (optional)
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

; Startup (optional)
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: startupicon

[Run]
; Option to launch after install
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Clean up any leftover files in install dir (NOT user data in AppData)
Type: filesandordirs; Name: "{app}"

[Code]
// Custom code to check for WebView2 runtime
function InitializeSetup(): Boolean;
var
  RegKey: String;
  Version: String;
begin
  Result := True;

  // Check if WebView2 Runtime is installed
  RegKey := 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  if not RegQueryStringValue(HKLM, RegKey, 'pv', Version) then
  begin
    RegKey := 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
    if not RegQueryStringValue(HKLM, RegKey, 'pv', Version) then
    begin
      if MsgBox('WebView2 Runtime is required but not installed.' + #13#10 + #13#10 +
                'Would you like to continue anyway? (The app may not work correctly)',
                mbConfirmation, MB_YESNO) = IDNO then
      begin
        Result := False;
      end;
    end;
  end;
end;

// Offer to delete user data on uninstall
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  UserDataDir: String;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    UserDataDir := ExpandConstant('{userappdata}\OpenSmurfManager');
    if DirExists(UserDataDir) then
    begin
      if MsgBox('Do you want to delete your saved accounts and settings?' + #13#10 + #13#10 +
                'Location: ' + UserDataDir + #13#10 + #13#10 +
                'WARNING: This cannot be undone!',
                mbConfirmation, MB_YESNO or MB_DEFBUTTON2) = IDYES then
      begin
        DelTree(UserDataDir, True, True, True);
      end;
    end;
  end;
end;
