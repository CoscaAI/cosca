package pipeline

import (
	"fmt"
	"strings"
)

type OpClassifier struct {
	patterns map[OpLevel][]string
}

type OpConfirmation struct {
	Operation string  `json:"operation"`
	Level     OpLevel `json:"level"`
	Reason    string  `json:"reason"`
	Required  bool    `json:"required"`
	AutoBlock bool    `json:"auto_block"`
}

func NewOpClassifier() *OpClassifier {
	return &OpClassifier{
		patterns: map[OpLevel][]string{
			OpRead: {
				"git status", "git log", "git diff", "git blame",
				"ls", "cat", "head", "tail", "grep", "find",
				"wc", "du", "df", "file", "which", "type",
				"echo", "printf", "pwd", "whoami", "env",
				"git show", "git branch", "git remote", "git tag",
			},
			OpWrite: {
				"git add", "mkdir", "touch",
				"echo >", "printf >", "cat >",
				"cp", "ln -s",
				"git commit", "git stash",
				"git branch -m", "git tag -a",
			},
			OpDangerous: {
				"rm ", "git reset", "git checkout",
				"mv ", "chmod ", "chown ",
				"git clean", "git rebase",
				"git branch -D", "git tag -d",
				"> ", "truncate", "shred",
			},
			OpSystem: {
				"apt ", "apt-get ", "yum ", "dnf ", "pacman ",
				"pip ", "pip3 ", "npm ", "yarn ", "pnpm ",
				"systemctl ", "service ", "docker ",
				"make install", "cmake --install",
				"brew ", "snap ", "flatpak ",
				"cargo install", "gem install",
				"mount ", "umount ", "losetup",
			},
			OpIrreversible: {
				"rm -rf", "rm -r ", "rmdir ",
				"git push --force", "git push -f",
				"drop ", "truncate ",
				"format ", "dd if=", "dd of=", "mkfs.",
				"git branch -D", "git reset --hard",
				":(){", "fork bomb",
				"> /dev/sd", "shutdown",
				"reboot", "poweroff", "halt",
			},
		},
	}
}

func (c *OpClassifier) Classify(command string) OpLevel {
	lower := strings.ToLower(strings.TrimSpace(command))

	for _, pattern := range c.patterns[OpIrreversible] {
		if strings.Contains(lower, pattern) {
			return OpIrreversible
		}
	}

	for _, pattern := range c.patterns[OpSystem] {
		if strings.Contains(lower, pattern) {
			return OpSystem
		}
	}

	for _, pattern := range c.patterns[OpDangerous] {
		if strings.Contains(lower, pattern) {
			return OpDangerous
		}
	}

	for _, pattern := range c.patterns[OpWrite] {
		if strings.Contains(lower, pattern) {
			return OpWrite
		}
	}

	for _, pattern := range c.patterns[OpRead] {
		if strings.Contains(lower, pattern) {
			return OpRead
		}
	}

	return OpRead
}

func (c *OpClassifier) RequireConfirmation(level OpLevel) *OpConfirmation {
	conf := &OpConfirmation{
		Level: level,
	}

	switch level {
	case OpRead:
		conf.Required = false
		conf.Operation = "read"
		conf.Reason = "read-only operations do not require confirmation"
	case OpWrite:
		conf.Required = false
		conf.Operation = "safe_write"
		conf.Reason = "safe write operations do not require confirmation"
	case OpModify:
		conf.Required = false
		conf.Operation = "modify"
		conf.Reason = "modification operations do not require confirmation"
	case OpDangerous:
		conf.Required = true
		conf.Operation = "dangerous_write"
		conf.Reason = "this operation modifies or removes existing data — review carefully"
	case OpSystem:
		conf.Required = true
		conf.Operation = "system"
		conf.Reason = "system operations can affect the entire system environment — review carefully"
	case OpIrreversible:
		conf.Required = true
		conf.AutoBlock = true
		conf.Operation = "irreversible"
		conf.Reason = "this operation is irreversible and cannot be undone — explicit override required"
	}

	return conf
}

func (c *OpClassifier) IsBlocked(level OpLevel) bool {
	return level == OpIrreversible
}

type SecurityGate struct {
	classifier *OpClassifier
	blocked    []string
}

func NewSecurityGate() *SecurityGate {
	return &SecurityGate{
		classifier: NewOpClassifier(),
		blocked:    make([]string, 0),
	}
}

func (g *SecurityGate) Evaluate(operation string) (*OpConfirmation, error) {
	level := g.classifier.Classify(operation)
	conf := g.classifier.RequireConfirmation(level)

	if g.classifier.IsBlocked(level) {
		g.blocked = append(g.blocked, operation)
		return conf, fmt.Errorf("operation blocked: %s — %s", conf.Reason, operation)
	}

	return conf, nil
}

func (g *SecurityGate) Override(operation string) (*OpConfirmation, error) {
	level := g.classifier.Classify(operation)
	conf := g.classifier.RequireConfirmation(level)

	if level == OpIrreversible || level == OpDangerous {
		conf.AutoBlock = false
		return conf, nil
	}

	return conf, fmt.Errorf("override not applicable for %s operations", level.String())
}

func (g *SecurityGate) BlockedOperations() []string {
	out := make([]string, len(g.blocked))
	copy(out, g.blocked)
	return out
}

func (g *SecurityGate) CanProceed(operation string) bool {
	conf, err := g.Evaluate(operation)
	if err != nil || conf.AutoBlock {
		return false
	}
	return true
}
