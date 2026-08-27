package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// ProjectArtifacts returns the always-on project instructions and, when
// requested, the skill's optional hooks and integrations as one install set.
func ProjectArtifacts(resolved ResolvedSkill, includeOptional bool) ResolvedSkill {
	selected := resolved
	selected.Artifacts = map[string][]ResolvedArtifact{}
	for target, instructions := range resolved.Instructions {
		selected.Artifacts[target] = append(selected.Artifacts[target], instructions...)
	}
	if includeOptional {
		for target, artifacts := range resolved.Artifacts {
			selected.Artifacts[target] = append(selected.Artifacts[target], artifacts...)
		}
	}
	return selected
}

func InstallArtifacts(
	resolved ResolvedSkill,
	targets []Target,
	projectRoot string,
	previous []state.LockArtifact,
	force bool,
) ([]state.LockArtifact, error) {
	type desiredArtifact struct {
		destination string
		mode        string
	}
	installations := deduplicateMarkdownInstallations(artifactInstallations(resolved, targets))
	desired := map[string]desiredArtifact{}
	for _, installation := range installations {
		artifact := installation.artifact
		key := artifactKey(installation.targetName, artifact.ID)
		if _, duplicate := desired[key]; duplicate {
			return nil, fmt.Errorf("artifact id %q is repeated for target %s", artifact.ID, installation.targetName)
		}
		destination, err := safeArtifactDestination(projectRoot, artifact.Destination)
		if err != nil {
			return nil, fmt.Errorf("install artifact %q: %w", artifact.ID, err)
		}
		desired[key] = desiredArtifact{destination: destination, mode: artifact.Mode}
	}

	previousByKey := map[string]state.LockArtifact{}
	var stale []state.LockArtifact
	for _, artifact := range previous {
		key := artifactKey(artifact.Target, artifact.ID)
		next, retained := desired[key]
		if !retained || next.destination != artifact.Destination || next.mode != artifact.Mode {
			stale = append(stale, artifact)
			continue
		}
		previousByKey[key] = artifact
	}
	if err := RemoveArtifacts(stale, projectRoot, force); err != nil {
		return nil, err
	}

	var installed []state.LockArtifact
	for _, installation := range installations {
		artifact := installation.artifact
		targetName := installation.targetName
		destination, err := safeArtifactDestination(projectRoot, artifact.Destination)
		if err != nil {
			return nil, fmt.Errorf("install artifact %q: %w", artifact.ID, err)
		}
		previousArtifact, hadPrevious := previousByKey[artifactKey(targetName, artifact.ID)]
		entry := state.LockArtifact{
			ID: artifact.ID, Target: targetName, Destination: destination,
			Mode: artifact.Mode, Digest: artifact.Digest,
		}
		_, destinationErr := os.Lstat(destination)
		entry.Created = os.IsNotExist(destinationErr)
		if hadPrevious {
			entry.Created = previousArtifact.Created
		}
		entry.Digest, err = DigestFile(artifact.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("digest artifact %q: %w", artifact.ID, err)
		}
		switch artifact.Mode {
		case state.ArtifactModeCopy:
			err = installCopiedArtifact(artifact.SourcePath, destination, artifact.Executable, previousArtifact, hadPrevious, force)
		case state.ArtifactModeMarkdownSection:
			entry.Marker = artifactMarkerPrefix(resolved.Name, targetName, artifact.ID)
			entry.Digest, err = markdownArtifactDigest(artifact.SourcePath)
			if err != nil {
				break
			}
			entry.MarkdownSeparator, err = installMarkdownArtifact(
				resolved.Name,
				artifact,
				targetName,
				destination,
				previousArtifact,
				hadPrevious,
				force,
			)
		case state.ArtifactModeJSONMerge:
			entry.ManagedJSON, err = installJSONArtifact(artifact, destination, previousArtifact, hadPrevious, force)
		default:
			err = fmt.Errorf("unknown artifact mode %q", artifact.Mode)
		}
		if err != nil {
			return nil, fmt.Errorf("install artifact %q for %s: %w", artifact.ID, targetName, err)
		}
		installed = append(installed, entry)
	}
	return installed, nil
}

type artifactInstallation struct {
	targetName string
	artifact   ResolvedArtifact
}

func artifactInstallations(resolved ResolvedSkill, targets []Target) []artifactInstallation {
	var installations []artifactInstallation
	if len(targets) > 0 {
		for _, artifact := range resolved.Artifacts["all"] {
			installations = append(installations, artifactInstallation{
				targetName: "all",
				artifact:   artifact,
			})
		}
	}
	for _, target := range targets {
		for _, artifact := range resolved.Artifacts[target.Name] {
			installations = append(installations, artifactInstallation{
				targetName: target.Name,
				artifact:   artifact,
			})
		}
	}
	return installations
}

// deduplicateMarkdownInstallations collapses groups of identical
// markdown-section artifacts — same destination, artifact id, and source
// digest — into a single installation under the shared "all" target. Catalogs
// commonly declare one lightweight pointer per target for files like
// AGENTS.md that many agents read; without this, the same section would be
// inlined once per selected target. Groups with a single member keep their
// per-target marker so existing installs are not churned, and groups whose
// sources differ keep separate per-target sections.
func deduplicateMarkdownInstallations(installations []artifactInstallation) []artifactInstallation {
	type markdownGroup struct {
		digests     map[string]struct{}
		destination string
		artifact    ResolvedArtifact
		members     int
		firstIndex  int
		collapsed   bool
	}
	groups := map[string]*markdownGroup{}
	var ordered []*markdownGroup
	for index, installation := range installations {
		artifact := installation.artifact
		if artifact.Mode != state.ArtifactModeMarkdownSection {
			continue
		}
		key := artifact.Destination + "\x00" + artifact.ID
		group, seen := groups[key]
		if !seen {
			group = &markdownGroup{
				destination: artifact.Destination,
				artifact:    artifact,
				digests:     map[string]struct{}{},
				firstIndex:  index,
			}
			groups[key] = group
			ordered = append(ordered, group)
		}
		group.digests[artifact.Digest] = struct{}{}
		group.members++
	}
	claimedAllKeys := map[string]string{}
	for _, group := range ordered {
		if group.members < 2 || len(group.digests) != 1 {
			continue
		}
		// Two collapsible groups that share an artifact id but target
		// different destinations would both claim the ("all", id) key;
		// keep the first and leave the rest per-target.
		if destination, claimed := claimedAllKeys[group.artifact.ID]; claimed && destination != group.destination {
			continue
		}
		claimedAllKeys[group.artifact.ID] = group.destination
		group.collapsed = true
	}
	var deduplicated []artifactInstallation
	for index, installation := range installations {
		artifact := installation.artifact
		if artifact.Mode == state.ArtifactModeMarkdownSection {
			if group, seen := groups[artifact.Destination+"\x00"+artifact.ID]; seen && group.collapsed {
				if index == group.firstIndex {
					deduplicated = append(deduplicated, artifactInstallation{targetName: "all", artifact: group.artifact})
				}
				continue
			}
		}
		deduplicated = append(deduplicated, installation)
	}
	return deduplicated
}

func RemoveArtifacts(artifacts []state.LockArtifact, projectRoot string, force bool) error {
	for _, artifact := range artifacts {
		destination, err := validateLockedDestination(projectRoot, artifact.Destination)
		if err != nil {
			return fmt.Errorf("remove artifact %q: %w", artifact.ID, err)
		}
		switch artifact.Mode {
		case state.ArtifactModeCopy:
			err = removeCopiedArtifact(destination, artifact, force)
		case state.ArtifactModeMarkdownSection:
			err = removeMarkdownArtifact(destination, artifact, force)
		case state.ArtifactModeJSONMerge:
			err = removeJSONArtifact(destination, artifact, force)
		default:
			err = fmt.Errorf("unknown artifact mode %q", artifact.Mode)
		}
		if err != nil {
			return fmt.Errorf("remove artifact %q for %s: %w", artifact.ID, artifact.Target, err)
		}
	}
	return nil
}

func installCopiedArtifact(source, destination string, executable bool, previous state.LockArtifact, hadPrevious, force bool) error {
	if _, err := os.Lstat(destination); err == nil {
		currentDigest, digestErr := DigestFile(destination)
		managed := digestErr == nil && hadPrevious && previous.Mode == state.ArtifactModeCopy && currentDigest == previous.Digest
		if !managed && !force {
			return errors.New("destination is unmanaged or locally modified; use --force to replace it")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	// #nosec G304 -- source path comes from the resolved artifact
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if executable {
		mode = 0o755
	}
	return state.WriteFileAtomic(destination, content, mode)
}

func removeCopiedArtifact(destination string, artifact state.LockArtifact, force bool) error {
	if _, err := os.Lstat(destination); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	digest, err := DigestFile(destination)
	if (err != nil || digest != artifact.Digest) && !force {
		return errors.New("destination is locally modified; use --force")
	}
	return os.Remove(destination)
}

func installMarkdownArtifact(
	skillName string,
	artifact ResolvedArtifact,
	targetName, destination string,
	previous state.LockArtifact,
	hadPrevious, force bool,
) (string, error) {
	source, err := os.ReadFile(artifact.SourcePath)
	if err != nil {
		return "", err
	}
	existing, err := readOptionalFile(destination)
	if err != nil {
		return "", err
	}
	start, end := artifactMarkers(skillName, targetName, artifact.ID)
	current, found, err := markedSection(existing, start, end)
	if err != nil {
		return "", err
	}
	if found && hadPrevious && previous.Mode == state.ArtifactModeMarkdownSection &&
		digestBytes(current) != previous.Digest && !force {
		return "", errors.New("managed Markdown section is locally modified; use --force")
	}
	block := renderMarkedSection(start, end, source)
	var updated []byte
	var separator string
	if found {
		updated = replaceMarkedSection(existing, start, end, block)
		separator = previous.MarkdownSeparator
	} else {
		separator = markdownSectionSeparator(existing)
		updated = append([]byte(nil), existing...)
		updated = append(updated, separator...)
		updated = append(updated, block...)
	}
	return separator, state.WriteFileAtomic(destination, updated, 0o644)
}

func removeMarkdownArtifact(destination string, artifact state.LockArtifact, force bool) error {
	existing, err := readOptionalFile(destination)
	if err != nil || existing == nil {
		return err
	}
	start, end := artifactMarkersFromLock(artifact)
	current, found, err := markedSection(existing, start, end)
	if err != nil || !found {
		return err
	}
	if digestBytes(current) != artifact.Digest && !force {
		return errors.New("managed Markdown section is locally modified; use --force")
	}
	updated, err := removeMarkedSection(existing, start, end, artifact.MarkdownSeparator, force)
	if err != nil {
		return err
	}
	if artifact.MarkdownSeparator == "" && len(bytes.TrimSpace(updated)) == 0 {
		updated = bytes.TrimRight(updated, "\n")
	}
	if artifact.Created && len(bytes.TrimSpace(updated)) == 0 {
		return os.Remove(destination)
	}
	return state.WriteFileAtomic(destination, updated, 0o644)
}

func installJSONArtifact(
	artifact ResolvedArtifact,
	destination string,
	previous state.LockArtifact,
	hadPrevious, force bool,
) (json.RawMessage, error) {
	source, err := os.ReadFile(artifact.SourcePath)
	if err != nil {
		return nil, err
	}
	fragment, canonical, err := decodeJSONObject(source)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	existingBytes, err := readOptionalFile(destination)
	if err != nil {
		return nil, err
	}
	existing := map[string]any{}
	if len(bytes.TrimSpace(existingBytes)) > 0 {
		if unmarshalErr := json.Unmarshal(existingBytes, &existing); unmarshalErr != nil {
			return nil, fmt.Errorf("decode destination JSON: %w", unmarshalErr)
		}
	}
	if hadPrevious && len(previous.ManagedJSON) > 0 {
		oldFragment, _, decodeErr := decodeJSONObject(previous.ManagedJSON)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode previous managed JSON: %w", decodeErr)
		}
		if !jsonFragmentMatches(existing, oldFragment) && !force {
			return nil, errors.New("managed JSON entries are locally modified; use --force")
		}
		removeJSONFragment(existing, oldFragment, force)
	} else if jsonFragmentConflicts(existing, fragment) && !force {
		return nil, errors.New("JSON destination contains conflicting unmanaged entries; use --force")
	}
	deepMergeJSON(existing, fragment)
	updated, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, err
	}
	updated = append(updated, '\n')
	if err := state.WriteFileAtomic(destination, updated, 0o644); err != nil {
		return nil, err
	}
	return canonical, nil
}

func removeJSONArtifact(destination string, artifact state.LockArtifact, force bool) error {
	existingBytes, err := readOptionalFile(destination)
	if err != nil || existingBytes == nil {
		return err
	}
	existing, _, err := decodeJSONObject(existingBytes)
	if err != nil {
		return fmt.Errorf("decode destination JSON: %w", err)
	}
	fragment, _, err := decodeJSONObject(artifact.ManagedJSON)
	if err != nil {
		return fmt.Errorf("decode managed JSON: %w", err)
	}
	if !jsonFragmentMatches(existing, fragment) && !force {
		return errors.New("managed JSON entries are locally modified; use --force")
	}
	removeJSONFragment(existing, fragment, force)
	if artifact.Created && len(existing) == 0 {
		return os.Remove(destination)
	}
	updated, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteFileAtomic(destination, append(updated, '\n'), 0o644)
}

func safeArtifactDestination(root, relative string) (string, error) {
	destination := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	if err := ensureContained(root, destination); err != nil {
		return "", err
	}
	current := root
	relative, err := filepath.Rel(root, destination)
	if err != nil {
		return "", err
	}
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("destination traverses symlink %s", current)
		}
	}
	if info, err := os.Lstat(destination); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("destination is a symlink: %s", destination)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return destination, nil
}

func validateLockedDestination(root, destination string) (string, error) {
	destination = filepath.Clean(destination)
	if err := ensureContained(root, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func ensureContained(root, path string) error {
	root = filepath.Clean(root)
	if path == root || !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return errors.New("destination escapes the project root")
	}
	return nil
}

func artifactKey(target, id string) string {
	return target + "\x00" + id
}

func artifactMarkers(skill, target, id string) (string, string) {
	prefix := artifactMarkerPrefix(skill, target, id)
	return "<!-- " + prefix + ":start -->", "<!-- " + prefix + ":end -->"
}

func artifactMarkersFromLock(artifact state.LockArtifact) (string, string) {
	prefix := artifact.Marker
	return "<!-- " + prefix + ":start -->", "<!-- " + prefix + ":end -->"
}

func artifactMarkerPrefix(skill, target, id string) string {
	return "repertoire:" + installDirectoryName(skill) + ":" + target + ":" + id
}

func renderMarkedSection(start, end string, content []byte) []byte {
	body := bytes.TrimSpace(content)
	result := append([]byte(start+"\n"), body...)
	result = append(result, []byte("\n"+end+"\n")...)
	return result
}

func markedSection(content []byte, start, end string) ([]byte, bool, error) {
	startIndex := bytes.Index(content, []byte(start))
	endIndex := bytes.Index(content, []byte(end))
	if startIndex < 0 && endIndex < 0 {
		return nil, false, nil
	}
	if startIndex < 0 || endIndex < 0 || endIndex < startIndex {
		return nil, false, errors.New("managed Markdown markers are malformed")
	}
	bodyStart := startIndex + len(start)
	return bytes.TrimSpace(content[bodyStart:endIndex]), true, nil
}

func replaceMarkedSection(content []byte, start, end string, replacement []byte) []byte {
	startIndex := bytes.Index(content, []byte(start))
	endIndex := bytes.Index(content, []byte(end)) + len(end)
	if startIndex < 0 || endIndex < len(end) {
		return content
	}
	for endIndex < len(content) && content[endIndex] == '\n' {
		endIndex++
	}
	result := append([]byte(nil), content[:startIndex]...)
	result = append(result, replacement...)
	result = append(result, content[endIndex:]...)
	return result
}

func markdownSectionSeparator(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	if content[len(content)-1] == '\n' {
		return "\n"
	}
	return "\n\n"
}

func removeMarkedSection(content []byte, start, end, separator string, force bool) ([]byte, error) {
	startIndex := bytes.Index(content, []byte(start))
	endIndex := bytes.Index(content, []byte(end)) + len(end)
	if startIndex < 0 || endIndex < len(end) {
		return content, nil
	}
	if separator != "" {
		if separator != "\n" && separator != "\n\n" {
			return nil, errors.New("managed Markdown separator is invalid")
		}
		if !bytes.HasSuffix(content[:startIndex], []byte(separator)) {
			// A block at the very start of the file has no preceding
			// separator, whatever the lock recorded at install time —
			// removing an adjacent earlier block consumes it.
			if startIndex != 0 && !force {
				return nil, errors.New("managed Markdown separator is locally modified; use --force")
			}
		} else {
			startIndex -= len(separator)
		}
	} else if startIndex >= 2 && bytes.Equal(content[startIndex-2:startIndex], []byte("\n\n")) {
		// Locks written before markdown_separator normalized user content to one
		// trailing newline plus one blank separator line.
		startIndex--
	}
	for endIndex < len(content) && content[endIndex] == '\n' {
		endIndex++
	}
	result := append([]byte(nil), content[:startIndex]...)
	result = append(result, content[endIndex:]...)
	return result, nil
}

func readOptionalFile(path string) ([]byte, error) {
	// #nosec G304 -- path comes from the resolved skill, not user input
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return content, err
}

func digestBytes(content []byte) string {
	sum := sha256.Sum256(bytes.TrimSpace(content))
	return hex.EncodeToString(sum[:])
}

func markdownArtifactDigest(path string) (string, error) {
	// #nosec G304 -- path is the resolved artifact destination
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digestBytes(content), nil
}

func decodeJSONObject(content []byte) (map[string]any, json.RawMessage, error) {
	var value map[string]any
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil, errors.New("expected a JSON object")
	}
	if err := json.Unmarshal(content, &value); err != nil {
		return nil, nil, err
	}
	canonical, err := json.Marshal(value)
	return value, canonical, err
}

func deepMergeJSON(destination, fragment map[string]any) {
	for key, value := range fragment {
		sourceObject, sourceIsObject := value.(map[string]any)
		destinationObject, destinationIsObject := destination[key].(map[string]any)
		if sourceIsObject && destinationIsObject {
			deepMergeJSON(destinationObject, sourceObject)
			continue
		}
		sourceArray, sourceIsArray := value.([]any)
		destinationArray, destinationIsArray := destination[key].([]any)
		if sourceIsArray && destinationIsArray {
			for _, item := range sourceArray {
				if !jsonArrayContains(destinationArray, item) {
					destinationArray = append(destinationArray, item)
				}
			}
			destination[key] = destinationArray
			continue
		}
		destination[key] = value
	}
}

func jsonFragmentMatches(destination, fragment map[string]any) bool {
	for key, value := range fragment {
		current, exists := destination[key]
		if !exists {
			return false
		}
		sourceObject, sourceIsObject := value.(map[string]any)
		destinationObject, destinationIsObject := current.(map[string]any)
		if sourceIsObject {
			if !destinationIsObject || !jsonFragmentMatches(destinationObject, sourceObject) {
				return false
			}
			continue
		}
		sourceArray, sourceIsArray := value.([]any)
		destinationArray, destinationIsArray := current.([]any)
		if sourceIsArray {
			if !destinationIsArray {
				return false
			}
			for _, item := range sourceArray {
				if !jsonArrayContains(destinationArray, item) {
					return false
				}
			}
			continue
		}
		if !jsonValuesEqual(current, value) {
			return false
		}
	}
	return true
}

func jsonFragmentConflicts(destination, fragment map[string]any) bool {
	for key, value := range fragment {
		current, exists := destination[key]
		if !exists {
			continue
		}
		sourceObject, sourceIsObject := value.(map[string]any)
		destinationObject, destinationIsObject := current.(map[string]any)
		if sourceIsObject && destinationIsObject {
			if jsonFragmentConflicts(destinationObject, sourceObject) {
				return true
			}
			continue
		}
		if _, sourceIsArray := value.([]any); sourceIsArray {
			if _, destinationIsArray := current.([]any); destinationIsArray {
				continue
			}
		}
		if !jsonValuesEqual(current, value) {
			return true
		}
	}
	return false
}

func removeJSONFragment(destination, fragment map[string]any, force bool) {
	for key, value := range fragment {
		current, exists := destination[key]
		if !exists {
			continue
		}
		sourceObject, sourceIsObject := value.(map[string]any)
		destinationObject, destinationIsObject := current.(map[string]any)
		if sourceIsObject && destinationIsObject {
			removeJSONFragment(destinationObject, sourceObject, force)
			if len(destinationObject) == 0 {
				delete(destination, key)
			}
			continue
		}
		sourceArray, sourceIsArray := value.([]any)
		destinationArray, destinationIsArray := current.([]any)
		if sourceIsArray && destinationIsArray {
			filtered := destinationArray[:0]
			for _, item := range destinationArray {
				if !jsonArrayContains(sourceArray, item) {
					filtered = append(filtered, item)
				}
			}
			if len(filtered) == 0 {
				delete(destination, key)
			} else {
				destination[key] = filtered
			}
			continue
		}
		if force || jsonValuesEqual(current, value) {
			delete(destination, key)
		}
	}
}

func jsonValuesEqual(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return bytes.Equal(leftJSON, rightJSON)
}

func jsonArrayContains(values []any, candidate any) bool {
	for _, value := range values {
		if jsonValuesEqual(value, candidate) {
			return true
		}
	}
	return false
}
