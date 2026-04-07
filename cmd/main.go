package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	embeddedscripts "toolbox/scripts"
)

type commandSource string

const (
	sourceDirect     commandSource = "Direct"
	sourcePowerShell commandSource = "PowerShell"
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
	{
		Title:       "install",
		Description: "의존성 설치",
		Command:     newPowerShellScriptCommand("scripts/powershell/install-dependencies.ps1"),
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

func resolveCommand(command toolCommand) (string, []string, error) {
	switch command.Source {
	case sourceDirect:
		if command.Program == "" {
			return "", nil, fmt.Errorf("직접 실행 명령의 프로그램 이름이 비어 있습니다")
		}

		path, err := resolveExecutablePath(command.Program)
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
		path, err := resolveExecutablePath(candidate)
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
		return "", fmt.Errorf("경로가 파일이 아닙니다: %s", path)
	}

	resolvedPath, err := filepath.Abs(path)
	if err == nil {
		return resolvedPath, nil
	}

	return path, nil
}

func describeCommand(command toolCommand) string {
	if command.Source == sourceDirect {
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
