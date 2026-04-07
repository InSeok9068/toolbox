package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	embeddedscripts "toolbox/scripts"
)

type commandSource string

const (
	sourceDirect     commandSource = "Direct"
	sourcePowerShell commandSource = "PowerShell"
	sourceInternal   commandSource = "Internal"
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

var options = []menuOption{
	{
		Title:       "rg",
		Description: "문자열 검색",
		Command:     newInternalCommand("fzf-rg"),
	},
	{
		Title:       "fd",
		Description: "파일 검색",
		Command:     newInternalCommand("fzf-fd"),
	},
	{
		Title:       "lazygit",
		Description: "Git TUI",
		Command:     newDirectCommand("lazygit"),
	},
	{
		Title:       "install",
		Description: "의존성 설치",
		Command:     newPowerShellScriptCommand("scripts/powershell/install-dependencies.ps1"),
	},
}

func main() {
	defer cleanupEmbeddedScripts()

	if handled, err := runInternalEntry(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	return &cobra.Command{
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
}

func promptOption() (menuOption, error) {
	labels := make([]string, 0, len(options))
	for index, option := range options {
		labels = append(labels, formatOptionLabel(index, option))
	}

	var selectedLabel string
	prompt := &survey.Select{
		Message: "실행할 도구를 선택하세요:",
		Options: labels,
	}

	if err := survey.AskOne(prompt, &selectedLabel); err != nil {
		return menuOption{}, err
	}

	for index, option := range options {
		if formatOptionLabel(index, option) == selectedLabel {
			return option, nil
		}
	}

	return menuOption{}, fmt.Errorf("선택한 도구를 찾을 수 없습니다: %s", selectedLabel)
}

func formatOptionLabel(index int, option menuOption) string {
	return fmt.Sprintf("%d. %s (%s)", index+1, option.Title, option.Description)
}

func newDirectCommand(program string, args ...string) toolCommand {
	return toolCommand{
		Source:  sourceDirect,
		Program: program,
		Args:    args,
	}
}

func newPowerShellScriptCommand(scriptPath string, args ...string) toolCommand {
	return toolCommand{
		Source:     sourcePowerShell,
		ScriptPath: scriptPath,
		Args:       args,
	}
}

func newInternalCommand(name string, args ...string) toolCommand {
	return toolCommand{
		Source:  sourceInternal,
		Program: name,
		Args:    args,
	}
}

func runTool(command toolCommand) error {
	if command.Source == sourceInternal {
		return runInternalCommand(command)
	}

	program, args, err := resolveCommand(command)
	if err != nil {
		return err
	}

	cmd := exec.Command(program, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s 실행 중 오류가 발생했습니다: %w", describeCommand(command), err)
	}

	return nil
}

func resolveCommand(command toolCommand) (string, []string, error) {
	switch command.Source {
	case sourceDirect:
		if command.Program == "" {
			return "", nil, fmt.Errorf("직접 실행 명령의 프로그램 이름이 비어 있습니다")
		}

		path, err := exec.LookPath(command.Program)
		if err != nil {
			return "", nil, fmt.Errorf("%q 실행 파일을 찾을 수 없습니다: %w", command.Program, err)
		}

		return path, command.Args, nil
	case sourcePowerShell:
		return resolvePowerShellCommand(command)
	case sourceInternal:
		return "", nil, fmt.Errorf("내부 명령은 직접 resolve할 수 없습니다: %s", command.Program)
	default:
		return "", nil, fmt.Errorf("지원하지 않는 실행 소스입니다: %s", command.Source)
	}
}

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

func findPowerShell() (string, error) {
	for _, candidate := range []string{"pwsh", "powershell"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s 실행 파일을 찾을 수 없습니다", sourcePowerShell)
}

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

func ensureFileExists(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return "", fmt.Errorf("스크립트 경로가 파일이 아닙니다: %s", path)
	}

	return path, nil
}

func describeCommand(command toolCommand) string {
	if command.Source == sourceDirect || command.Source == sourceInternal {
		return command.Program
	}

	if command.ScriptPath != "" {
		return command.ScriptPath
	}

	return string(command.Source)
}

func cleanupEmbeddedScripts() {
	if err := embeddedscripts.Cleanup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func runInternalEntry(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "__rg_reload__":
		return true, runRGReload(args[1:])
	default:
		return false, nil
	}
}

func runInternalCommand(command toolCommand) error {
	switch command.Program {
	case "fzf-rg":
		return runFzfRG()
	case "fzf-fd":
		return runFzfFD()
	default:
		return fmt.Errorf("알 수 없는 내부 명령입니다: %s", command.Program)
	}
}

func runFzfRG() error {
	if _, err := lookPathAll("fzf", "rg", "bat", "code"); err != nil {
		return err
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("현재 실행 파일 경로를 확인할 수 없습니다: %w", err)
	}

	quotedExecutable := fmt.Sprintf(`"%s"`, executablePath)
	reloadOnStart := fmt.Sprintf(`start:reload:%s __rg_reload__ ""`, quotedExecutable)
	reloadOnChange := fmt.Sprintf(`change:reload:%s __rg_reload__ "{q}"`, quotedExecutable)
	previewCommand := `bat --style=numbers --color=always --highlight-line {2} -- "{1}" 2>NUL`
	openCommand := `enter:become(code --goto "{1}:{2}:{3}")`

	args := []string{
		"--ansi",
		"--disabled",
		"--prompt", "rg> ",
		"--delimiter", ":",
		"--with-nth", "1,2,4..",
		"--bind", reloadOnStart,
		"--bind", reloadOnChange,
		"--bind", openCommand,
		"--preview", previewCommand,
		"--preview-window", "right:60%:wrap,+{2}/2",
	}

	return runFzf(args, nil)
}

func runFzfFD() error {
	fdPath, err := lookPathAll("fd", "fzf", "bat", "code")
	if err != nil {
		return err
	}

	previewCommand := `bat --color=always --style=numbers --line-range=:500 -- "{}" 2>NUL`
	openCommand := `enter:become(code "{}")`

	fzfArgs := []string{
		"--preview", previewCommand,
		"--preview-window", "right:60%:wrap",
		"--bind", "ctrl-/:toggle-preview",
		"--bind", openCommand,
	}

	fdCommand := exec.Command(fdPath[0],
		"--type", "f",
		"--hidden",
		"--exclude", ".git",
		"--exclude", "node_modules",
		"--exclude", "dist",
		"--exclude", "build",
		"--exclude", ".next",
		"--exclude", "coverage",
	)

	return runFzf(fzfArgs, fdCommand)
}

func runFzf(args []string, inputCommand *exec.Cmd) error {
	fzfPath, err := exec.LookPath("fzf")
	if err != nil {
		return fmt.Errorf("%q 실행 파일을 찾을 수 없습니다: %w", "fzf", err)
	}

	fzfCommand := exec.Command(fzfPath, args...)
	fzfCommand.Stdin = os.Stdin
	fzfCommand.Stdout = os.Stdout
	fzfCommand.Stderr = os.Stderr
	fzfCommand.Env = withShellOverride(os.Environ(), "cmd.exe")

	if inputCommand != nil {
		pipeReader, pipeWriter, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("fzf 입력 파이프를 준비할 수 없습니다: %w", err)
		}

		inputCommand.Stdout = pipeWriter
		inputCommand.Stderr = os.Stderr
		inputCommand.Env = os.Environ()

		fzfCommand.Stdin = pipeReader

		if err := inputCommand.Start(); err != nil {
			_ = pipeReader.Close()
			_ = pipeWriter.Close()
			return fmt.Errorf("%s 실행을 시작할 수 없습니다: %w", inputCommand.Path, err)
		}

		if err := fzfCommand.Start(); err != nil {
			_ = pipeReader.Close()
			_ = pipeWriter.Close()
			_ = inputCommand.Process.Kill()
			_ = inputCommand.Wait()
			return fmt.Errorf("%q 실행을 시작할 수 없습니다: %w", "fzf", err)
		}

		_ = pipeWriter.Close()
		_ = pipeReader.Close()

		inputErr := inputCommand.Wait()
		fzfErr := fzfCommand.Wait()

		if inputErr != nil && !isIgnorablePipeError(inputErr) {
			return fmt.Errorf("%s 실행 중 오류가 발생했습니다: %w", inputCommand.Path, inputErr)
		}

		if fzfErr != nil {
			return fmt.Errorf("%q 실행 중 오류가 발생했습니다: %w", "fzf", fzfErr)
		}

		return nil
	}

	if err := fzfCommand.Run(); err != nil {
		return fmt.Errorf("%q 실행 중 오류가 발생했습니다: %w", "fzf", err)
	}

	return nil
}

func runRGReload(args []string) error {
	query := ""
	if len(args) > 0 {
		query = strings.TrimSpace(args[0])
	}

	if query == "" {
		return nil
	}

	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return fmt.Errorf("%q 실행 파일을 찾을 수 없습니다: %w", "rg", err)
	}

	commandArgs := []string{
		"--column",
		"--line-number",
		"--no-heading",
		"--color=always",
		"--smart-case",
		"--hidden",
		"-g", "!.git",
		"-g", "!node_modules",
		"-g", "!dist",
		"-g", "!build",
		"-g", "!.next",
		"-g", "!coverage",
		query,
	}

	cmd := exec.Command(rgPath, commandArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}

		return fmt.Errorf("%q 실행 중 오류가 발생했습니다: %w", "rg", err)
	}

	return nil
}

func isIgnorablePipeError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "broken pipe") || strings.Contains(message, "pipe has been ended")
}

func withShellOverride(env []string, shell string) []string {
	updatedEnv := make([]string, 0, len(env)+1)
	replaced := false

	for _, item := range env {
		if len(item) >= 6 && item[:6] == "SHELL=" {
			updatedEnv = append(updatedEnv, "SHELL="+shell)
			replaced = true
			continue
		}

		updatedEnv = append(updatedEnv, item)
	}

	if !replaced {
		updatedEnv = append(updatedEnv, "SHELL="+shell)
	}

	return updatedEnv
}

func lookPathAll(names ...string) ([]string, error) {
	paths := make([]string, 0, len(names))

	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			return nil, fmt.Errorf("%q 실행 파일을 찾을 수 없습니다: %w", name, err)
		}

		paths = append(paths, path)
	}

	return paths, nil
}
