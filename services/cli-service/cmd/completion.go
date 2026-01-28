package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Генерировать скрипт автодополнения",
	Long: `Генерирует скрипт автодополнения для указанной оболочки.
Чтобы включить автодополнение:

Bash:
  $ source <(uptimeping completion bash)

  # Для постоянного использования:
  $ uptimeping completion bash > /etc/bash_completion.d/uptimeping
  $ uptimeping completion bash > ~/.local/share/bash-completion/completions/uptimeping

Zsh:
  # Если автодополнение еще не включено:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # Для постоянного использования:
  $ uptimeping completion zsh > "${fpath[1]}/_uptimeping"

  # Перезагрузить оболочку:
  $ exec $SHELL

Fish:
  $ uptimeping completion fish | source

  # Для постоянного использования:
  $ uptimeping completion fish > ~/.config/fish/completions/uptimeping.fish

PowerShell:
  PS> uptimeping completion powershell | Out-String | Invoke-Expression

  # Для постоянного использования:
  PS> uptimeping completion powershell > uptimeping.ps1
  PS> echo ". .\uptimeping.ps1" >> $PROFILE`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleCompletion(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

func handleCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]
	
	switch shell {
	case "bash":
		return rootCmd.GenBashCompletion(os.Stdout)
	case "zsh":
		return rootCmd.GenZshCompletion(os.Stdout)
	case "fish":
		return rootCmd.GenFishCompletion(os.Stdout, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
}

// setupAutoCompletion устанавливает автодополнение для CLI
func setupAutoCompletion() error {
	// Проверяем, установлена ли переменная окружения для автодополнения
	shell := os.Getenv("SHELL")
	if shell == "" {
		return nil // Не устанавливаем автодополнение если не определена оболочка
	}

	// Определяем тип оболочки
	var completionFunc func() error
	switch {
	case strings.Contains(shell, "bash"):
		completionFunc = func() error {
			return rootCmd.GenBashCompletionFile("/tmp/uptimeping-completion.bash")
		}
	case strings.Contains(shell, "zsh"):
		completionFunc = func() error {
			return rootCmd.GenZshCompletionFile("/tmp/uptimeping-completion.zsh")
		}
	case strings.Contains(shell, "fish"):
		completionFunc = func() error {
			return rootCmd.GenFishCompletionFile("/tmp/uptimeping-completion.fish", true)
		}
	default:
		return nil // Неподдерживаемая оболочка
	}

	// Генерируем файл автодополнения
	if completionFunc != nil {
		if err := completionFunc(); err != nil {
			return fmt.Errorf("failed to generate completion file: %w", err)
		}
	}

	return nil
}

// validateShell проверяет поддерживается ли указанная оболочка
func validateShell(shell string) bool {
	supportedShells := []string{"bash", "zsh", "fish", "powershell"}
	for _, supported := range supportedShells {
		if shell == supported {
			return true
		}
	}
	return false
}

// getCompletionInstructions возвращает инструкции по установке автодополнения
func getCompletionInstructions(shell string) string {
	switch shell {
	case "bash":
		return `# Временное включение:
source <(uptimeping completion bash)

# Постоянное использование (выберите один из вариантов):
# 1. Системный уровень:
sudo uptimeping completion bash > /etc/bash_completion.d/uptimeping

# 2. Пользовательский уровень:
mkdir -p ~/.local/share/bash-completion/completions
uptimeping completion bash > ~/.local/share/bash-completion/completions/uptimeping

# 3. Добавить в ~/.bashrc:
echo 'source <(uptimeping completion bash)' >> ~/.bashrc`

	case "zsh":
		return `# Если автодополнение еще не включено:
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Постоянное использование:
uptimeping completion zsh > ~/.zsh/completions/_uptimeping

# Или добавить в fpath:
mkdir -p ~/.zsh/completions
uptimeping completion zsh > ~/.zsh/completions/_uptimeping

# Перезагрузить оболочку:
exec $SHELL`

	case "fish":
		return `# Временное включение:
uptimeping completion fish | source

# Постоянное использование:
mkdir -p ~/.config/fish/completions
uptimeping completion fish > ~/.config/fish/completions/uptimeping.fish

# Перезагрузить оболочку:
exec $SHELL`

	case "powershell":
		return `# Временное включение:
uptimeping completion powershell | Out-String | Invoke-Expression

# Постоянное использование:
uptimeping completion powershell > $PROFILE\..\uptimeping.ps1
echo ". $PROFILE\..\uptimeping.ps1" >> $PROFILE

# Или добавить в профиль:
Add-Content -Path $PROFILE -Value (uptimeping completion powershell)`

	default:
		return "Unsupported shell"
	}
}
