# toolbox

CLI Toolbox

## 빌드

```shell
go build -o a.exe ./cmd
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

`toolbox`는 실행 파일 옆의 `bin/`에 들어 있는 바이너리를 직접 사용합니다.
