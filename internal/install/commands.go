package install

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// Some clients list user-invoked entries apart from skills: OpenCode reads
// commands from its own directory and offers them in the same palette as its
// built-in ones, while every skill it finds is offered to the model. A skill
// whose frontmatter marks it disable-model-invocation is meant to be invoked
// explicitly, so for a target with a command directory it is installed there as
// well.
//
// The skill directory is still written. A command is a single file, and skills
// carry companions — references, scripts, licences — that a command file has
// nowhere to put and no directory of its own to resolve against. Installing both
// keeps those reachable, at the cost of the client also seeing the skill.

// readSkillHeader reads a skill's SKILL.md and reports its header and body.
func readSkillHeader(root string) (catalog.SkillFrontmatter, []byte, error) {
	// #nosec G304 -- root is a resolved skill directory
	content, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil {
		return catalog.SkillFrontmatter{}, nil, fmt.Errorf("read skill SKILL.md: %w", err)
	}
	header, err := catalog.ParseSkillFrontmatter(content)
	if err != nil {
		return catalog.SkillFrontmatter{}, nil, err
	}
	body, err := skillBody(content)
	if err != nil {
		return catalog.SkillFrontmatter{}, nil, err
	}
	return header, body, nil
}

// skillBody returns everything after a SKILL.md's opening frontmatter block,
// splitting it the same way catalog.ParseSkillFrontmatter does.
func skillBody(content []byte) ([]byte, error) {
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		return nil, errors.New("invalid YAML frontmatter")
	}
	return []byte(parts[2]), nil
}

// renderCommand builds the command file for a slash-only skill: a header
// carrying only the description, then the SKILL.md body as the prompt template.
//
// The flag is taken from the frontmatter alone. A SKILL.md is free to mention
// disable-model-invocation in its prose, and reading the raw text would mistake
// such a skill for one the model must not load.
func renderCommand(header catalog.SkillFrontmatter, body []byte) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("---\n")
	fmt.Fprintf(&out, "description: %s\n", yamlScalar(header.Description))
	out.WriteString("---\n\n")
	out.WriteString(strings.TrimLeft(string(body), "\n"))
	if !strings.HasSuffix(out.String(), "\n") {
		out.WriteString("\n")
	}
	if strings.TrimSpace(header.ArgumentHint) != "" {
		// OpenCode substitutes the typed text only where the template names it,
		// and these bodies describe the argument in prose instead.
		out.WriteString("\n## Arguments\n\n")
		out.WriteString("`$ARGUMENTS` is the text typed after the command name, and is empty when none was.\n")
		out.WriteString("The steps above say what an empty value means.\n")
	}
	return out.Bytes(), nil
}

// yamlScalar quotes a description that would not survive being written bare.
// Descriptions routinely contain a colon, which ends a plain scalar.
func yamlScalar(value string) string {
	trimmed := strings.TrimSpace(value)
	if !strings.ContainsAny(trimmed, ":#\"'\n") {
		return trimmed
	}
	escaped := strings.ReplaceAll(trimmed, `\`, `\\`)
	return `"` + strings.ReplaceAll(escaped, `"`, `\"`) + `"`
}

// commandDestination is where a target keeps the command for one skill.
func commandDestination(target Target, name string) string {
	return filepath.Join(target.Commands, installDirectoryName(name)+".md")
}

// InstallCommands writes the command file for a slash-only skill for each target
// that reads commands, and returns the lock records for them. A target with no
// command directory is skipped, so this is a no-op for most targets.
//
// A command file is written with the single-file helpers: the skill copier
// stages a directory and renames it into place, and the directory digest walks
// the entries below a root, so neither describes a lone file.
func InstallCommands(
	resolved ResolvedSkill,
	targets []Target,
	previous []state.LockCommand,
	force bool,
) ([]state.LockCommand, error) {
	selected := targetsWithCommands(targets)
	if len(selected) == 0 {
		return nil, nil
	}
	header, body, err := readSkillHeader(resolved.Root)
	if err != nil {
		return nil, err
	}
	if !header.DisableModelInvocation {
		return nil, nil
	}
	rendered, err := renderCommand(header, body)
	if err != nil {
		return nil, fmt.Errorf("render command for %s: %w", resolved.Name, err)
	}
	locked := previousByDestination(previous)

	var installed []state.LockCommand
	for _, target := range selected {
		destination := commandDestination(target, resolved.Name)
		record, err := writeCommand(destination, rendered, locked[destination], force)
		if err != nil {
			return nil, err
		}
		record.Target = target.Name
		installed = append(installed, record)
	}
	return installed, nil
}

// RemoveCommands deletes the command files a previous install recorded.
func RemoveCommands(name string, commands []state.LockCommand, force bool) error {
	for _, command := range commands {
		exists, matches, err := DigestFileMatches(command.Destination, command.Digest)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if !matches && !force {
			return fmt.Errorf("remove %q from %s: command is locally modified; use --force", name, command.Target)
		}
		if err := os.Remove(command.Destination); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove command %s: %w", command.Destination, err)
		}
	}
	return nil
}

// writeCommand writes one command file, preserving a file that has been edited
// since the last install unless force is set, and reports the lock record for it.
func writeCommand(destination string, content []byte, previous state.LockCommand, force bool) (state.LockCommand, error) {
	record := state.LockCommand{
		Destination: destination,
		Digest:      digestFileContent(content),
	}
	exists, matches, err := DigestFileMatches(destination, previous.Digest)
	if err != nil {
		return state.LockCommand{}, err
	}
	if exists {
		managedAndUnchanged := previous.Digest != "" && matches
		if !managedAndUnchanged && !force {
			return state.LockCommand{}, fmt.Errorf("command %s is unmanaged or locally modified; use --force to replace it", destination)
		}
	}
	// #nosec G306 -- command files follow the shared 0644 convention
	if err := state.WriteFileAtomic(destination, content, 0o644); err != nil {
		return state.LockCommand{}, err
	}
	return record, nil
}

// PlanCommands classifies the command file a slash-only skill would write for
// each target that reads commands, without writing anything. It returns no plans
// for a skill that is not slash-only, which is the common case.
func PlanCommands(resolved ResolvedSkill, targets []Target, previous *state.LockSkill, force bool) ([]SkillCopyPlan, error) {
	selected := targetsWithCommands(targets)
	if len(selected) == 0 {
		return nil, nil
	}
	header, body, err := readSkillHeader(resolved.Root)
	if err != nil {
		return nil, err
	}
	if !header.DisableModelInvocation {
		return nil, nil
	}
	rendered, err := renderCommand(header, body)
	if err != nil {
		return nil, fmt.Errorf("render command for %s: %w", resolved.Name, err)
	}
	var locked map[string]state.LockCommand
	if previous != nil {
		locked = previousByDestination(previous.Commands)
	}
	desired := digestFileContent(rendered)
	plans := make([]SkillCopyPlan, 0, len(selected))
	for _, target := range selected {
		destination := commandDestination(target, resolved.Name)
		action, err := classifyInstallCommand(destination, locked[destination].Digest, desired, force)
		if err != nil {
			return nil, err
		}
		plans = append(plans, SkillCopyPlan{
			Skill: resolved.Name, Target: target.Name, Path: destination, Action: action,
		})
	}
	return plans, nil
}

// PlanRemoveCommands classifies the recorded command files a removal would
// delete, without deleting them. A command the user has edited is refused rather
// than reported for removal, matching how a modified skill copy is treated.
func PlanRemoveCommands(name string, previous state.LockSkill, force bool) ([]SkillCopyPlan, error) {
	var plans []SkillCopyPlan
	for _, command := range previous.Commands {
		exists, matches, err := DigestFileMatches(command.Destination, command.Digest)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		action := SkillCopyRemove
		if !matches && !force {
			action = SkillCopyRefuseModified
		}
		plans = append(plans, SkillCopyPlan{
			Skill: name, Target: command.Target, Path: command.Destination, Action: action,
		})
	}
	return plans, nil
}

// targetsWithCommands returns the targets that read user-invoked entries.
func targetsWithCommands(targets []Target) []Target {
	var selected []Target
	for _, target := range targets {
		if target.Commands != "" {
			selected = append(selected, target)
		}
	}
	return selected
}

// previousByDestination indexes the command records a previous install wrote.
func previousByDestination(previous []state.LockCommand) map[string]state.LockCommand {
	byDestination := make(map[string]state.LockCommand, len(previous))
	for _, command := range previous {
		byDestination[command.Destination] = command
	}
	return byDestination
}
