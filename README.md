# toolbox

CLI Toolbox

## 빌드

```shell
go build -o .\toolbox.exe ./cmd
```

## 릴리스 구조

릴리스 zip 루트에는 `toolbox.exe`와 `bin/`만 두는 형태를 기준으로 합니다.

```text
toolbox.exe
bin/
  rg.exe
  fd.exe
  fzf.exe
  bat.exe
  delta.exe
  lazygit.exe
```

`toolbox`는 실행 파일 옆의 `bin/`을 먼저 찾고, 없으면 시스템 `PATH`를 사용합니다.

## 릴리스 패키징

repo 루트에 `bin/`을 준비한 뒤 아래 스크립트로 release zip을 만들 수 있습니다.

```powershell
.\scripts\powershell\package-release.ps1
```
