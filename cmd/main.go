package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	embeddedscripts "toolbox/scripts"
)

type commandSource string

const (
	sourceDirect     commandSource = "Direct"
	sourcePowerShell commandSource = "PowerShell"
	toolboxBinDirEnv               = "TOOLBOX_BIN_DIR"
)

type menuOption struct {
	Title       string
	Description string
	Command     toolCommand
}

type toolCommand struct {
	Source     commandSource
	Program    string
	ScriptPath string
	Args       []string
}

// options는 대화형 메뉴에 보여줄 도구 목록을 정의한다.
var options = []menuOption{
	{
		Title:       "rg",
		Description: "문자열 검색",
		Command:     newPowerShellScriptCommand("scripts/powershell/fzf-rg.ps1"),
	},
	{
		Title:       "fd",
		Description: "파일 검색",
		Command:     newPowerShellScriptCommand("scripts/powershell/fzf-fd.ps1"),
	},
	{
		Title:       "lazygit",
		Description: "Git TUI",
		Command:     newDirectCommand("lazygit"),
	},
}

func main() {
	defer cleanupEmbeddedScripts()

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd는 대화형 메뉴와 단축 서브커맨드를 구성한다.
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "toolbox",
		Short:        "작업 도구 실행기",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, err := promptOption()
			if err != nil {
				return err
			}

			return runTool(selected.Command)
		},
	}

	rootCmd.AddCommand(newShortcutCommand("rg", "문자열 검색 바로 실행", newPowerShellScriptCommand("scripts/powershell/fzf-rg.ps1")))
	rootCmd.AddCommand(newShortcutCommand("fd", "파일 검색 바로 실행", newPowerShellScriptCommand("scripts/powershell/fzf-fd.ps1")))

	return rootCmd
}

// newShortcutCommand는 `toolbox rg` 같은 단축 서브커맨드를 만든다.
func newShortcutCommand(name string, short string, command toolCommand) *cobra.Command {
	return &cobra.Command{
		Use:          name,
		Short:        short,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTool(command)
		},
	}
}

// promptOption은 메뉴를 보여주고 선택한 라벨을 실제 명령으로 되돌린다.
func promptOption() (menuOption, error) {
	labels := make([]string, 0, len(options))
	optionsByLabel := make(map[string]menuOption, len(options))

	for index, option := range options {
		label := formatOptionLabel(index, option)
		labels = append(labels, label)
		optionsByLabel[label] = option
	}

	var selectedLabel string
	prompt := &survey.Select{
		Message: "실행할 도구를 선택하세요:",
		Options: labels,
	}

	if err := survey.AskOne(prompt, &selectedLabel); err != nil {
		return menuOption{}, err
	}

	selectedOption, exists := optionsByLabel[selectedLabel]
	if exists {
		return selectedOption, nil
	}

	return menuOption{}, fmt.Errorf("선택한 도구를 찾을 수 없습니다: %s", selectedLabel)
}

// formatOptionLabel은 사용자에게 보여줄 번호형 메뉴 라벨을 만든다.
func formatOptionLabel(index int, option menuOption) string {
	return fmt.Sprintf("%d. %s (%s)", index+1, option.Title, option.Description)
}

// newDirectCommand는 번들된 실행 파일용 도구 정의를 만든다.
func newDirectCommand(program string, args ...string) toolCommand {
	return toolCommand{
		Source:  sourceDirect,
		Program: program,
		Args:    args,
	}
}

// newPowerShellScriptCommand는 PowerShell 스크립트 진입점용 도구 정의를 만든다.
func newPowerShellScriptCommand(scriptPath string, args ...string) toolCommand {
	return toolCommand{
		Source:     sourcePowerShell,
		ScriptPath: scriptPath,
		Args:       args,
	}
}

// runTool은 해석된 명령을 실행하고 대화형 도구를 위해 표준 입출력을 그대로 연결한다.
func runTool(command toolCommand) error {
	program, args, err := resolveCommand(command)
	if err != nil {
		return err
	}

	cmd := exec.Command(program, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = commandEnvironment()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s 실행 중 오류가 발생했습니다: %w", describeCommand(command), err)
	}

	return nil
}

// resolveCommand는 도구 정의에 맞는 실행 방식을 선택한다.
func resolveCommand(command toolCommand) (string, []string, error) {
	switch command.Source {
	case sourceDirect:
		if command.Program == "" {
			return "", nil, fmt.Errorf("직접 실행 명령의 프로그램 이름이 비어 있습니다")
		}

		path, err := bundledExecutablePath(command.Program)
		if err != nil {
			return "", nil, err
		}

		return path, command.Args, nil
	case sourcePowerShell:
		return resolvePowerShellCommand(command)
	default:
		return "", nil, fmt.Errorf("지원하지 않는 실행 소스입니다: %s", command.Source)
	}
}

// describeCommand는 실행 오류 메시지에 사용할 식별자를 반환한다.
func describeCommand(command toolCommand) string {
	if command.Source == sourceDirect {
		return command.Program
	}

	if command.ScriptPath != "" {
		return command.ScriptPath
	}

	return string(command.Source)
}

// resolvePowerShellCommand는 PowerShell을 시스템 의존으로 두고 스크립트 경로만 해석한다.
func resolvePowerShellCommand(command toolCommand) (string, []string, error) {
	if command.ScriptPath == "" {
		return "", nil, fmt.Errorf("PowerShell 스크립트 경로가 비어 있습니다")
	}

	powerShellPath, err := findPowerShell()
	if err != nil {
		return "", nil, err
	}

	scriptPath, err := resolveScriptPath(command.ScriptPath)
	if err != nil {
		return "", nil, err
	}

	args := []string{
		"-NoLogo",
		"-NoProfile",
		"-ExecutionPolicy",
		"Bypass",
		"-File",
		scriptPath,
	}

	args = append(args, command.Args...)
	return powerShellPath, args, nil
}

// findPowerShell은 가능하면 PowerShell 7을 사용하고 없으면 Windows PowerShell로 돌아간다.
func findPowerShell() (string, error) {
	for _, candidate := range []string{"pwsh", "powershell"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s 실행 파일을 찾을 수 없습니다", sourcePowerShell)
}

// bundledExecutablePath는 `toolbox.exe` 옆 `bin/` 디렉터리에서 번들 도구 경로를 찾는다.
func bundledExecutablePath(program string) (string, error) {
	if strings.TrimSpace(program) == "" {
		return "", fmt.Errorf("실행할 프로그램 이름이 비어 있습니다")
	}

	binDir, err := bundledBinDir()
	if err != nil {
		return "", err
	}

	return ensureFileExists(filepath.Join(binDir, platformExecutableName(program)))
}

// bundledBinDir는 실행 파일 옆 `bin/`을 우선 사용하고, 없으면 현재 작업 디렉터리의 `bin/`을 본다.
func bundledBinDir() (string, error) {
	candidates := make([]string, 0, 2)

	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("실행 파일 경로를 확인할 수 없습니다: %w", err)
	}

	candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), "bin"))

	workingDirectory, err := os.Getwd()
	if err == nil {
		candidates = append(candidates, filepath.Join(workingDirectory, "bin"))
	}

	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			resolvedPath, absErr := filepath.Abs(candidate)
			if absErr == nil {
				return resolvedPath, nil
			}

			return candidate, nil
		}
	}

	return filepath.Abs(candidates[0])
}

// platformExecutableName은 Windows에서 확장자가 없을 때 `.exe`를 붙인다.
func platformExecutableName(name string) string {
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		return name + ".exe"
	}

	return name
}

// commandEnvironment는 PATH를 바꾸지 않고 자식 스크립트에 번들 bin 경로를 전달한다.
func commandEnvironment() []string {
	binDir, err := bundledBinDir()
	if err != nil {
		return os.Environ()
	}

	return upsertEnvironmentVariable(os.Environ(), toolboxBinDirEnv, binDir)
}

// upsertEnvironmentVariable은 같은 키가 있으면 값을 바꾸고 없으면 새로 추가한다.
func upsertEnvironmentVariable(environment []string, key string, value string) []string {
	environmentCopy := append([]string(nil), environment...)

	for index, environmentEntry := range environmentCopy {
		entryKey, _, found := strings.Cut(environmentEntry, "=")
		if !found || !matchesEnvironmentKey(entryKey, key) {
			continue
		}

		environmentCopy[index] = key + "=" + value
		return environmentCopy
	}

	return append(environmentCopy, key+"="+value)
}

// matchesEnvironmentKey는 Windows의 대소문자 규칙을 반영해 환경 변수 키를 비교한다.
func matchesEnvironmentKey(left string, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}

	return left == right
}

// resolveScriptPath는 로컬 파일, 실행 파일 옆 파일, 임베디드 스크립트 순서로 경로를 찾는다.
func resolveScriptPath(scriptPath string) (string, error) {
	if scriptPath == "" {
		return "", fmt.Errorf("스크립트 경로가 비어 있습니다")
	}

	if filepath.IsAbs(scriptPath) {
		return ensureFileExists(scriptPath)
	}

	candidates := make([]string, 0, 2)

	workingDirectory, err := os.Getwd()
	if err == nil {
		candidates = append(candidates, filepath.Join(workingDirectory, scriptPath))
	}

	executablePath, err := os.Executable()
	if err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), scriptPath))
	}

	for _, candidate := range candidates {
		resolvedPath, err := ensureFileExists(candidate)
		if err == nil {
			return resolvedPath, nil
		}
	}

	embeddedRoot, err := embeddedscripts.Extract()
	if err != nil {
		return "", fmt.Errorf("임베디드 스크립트를 준비할 수 없습니다: %w", err)
	}

	embeddedPath := filepath.Join(embeddedRoot, filepath.FromSlash(scriptPath))
	resolvedPath, err := ensureFileExists(embeddedPath)
	if err == nil {
		return resolvedPath, nil
	}

	return "", fmt.Errorf("스크립트 파일을 찾을 수 없습니다: %s", scriptPath)
}

// ensureFileExists는 경로가 실제 파일인지 확인하고 가능하면 절대경로로 정규화한다.
func ensureFileExists(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return "", fmt.Errorf("경로가 파일이 아닙니다: %s", path)
	}

	resolvedPath, err := filepath.Abs(path)
	if err == nil {
		return resolvedPath, nil
	}

	return path, nil
}

// cleanupEmbeddedScripts는 임베디드 스크립트에서 풀어낸 임시 파일을 정리한다.
func cleanupEmbeddedScripts() {
	if err := embeddedscripts.Cleanup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
