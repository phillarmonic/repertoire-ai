package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// conflictingDestination: more than one locked artifact manages the same
// file in incompatible ways — copy-mode entries with different content, or
// copy mode sharing a destination with other modes. Copy mode owns the whole
// file, so these can never all match at once; reinstalling one just breaks
// the other. Doctor reports the conflict and leaves resolution to the
// catalogs.
type conflictingDestination struct{}

func (conflictingDestination) id() string { return "conflicting-destination" }

// conflictingDestinations groups locked project artifacts by destination and
// returns those claimed incompatibly.
func conflictingDestinations(locked []lockedArtifact) map[string][]lockedArtifact {
	byDestination := map[string][]lockedArtifact{}
	for _, artifact := range locked {
		byDestination[artifact.artifact.Destination] = append(byDestination[artifact.artifact.Destination], artifact)
	}
	conflicts := map[string][]lockedArtifact{}
	for destination, entries := range byDestination {
		if len(entries) < 2 {
			continue
		}
		modes := map[string]bool{}
		copyDigests := map[string]bool{}
		for _, entry := range entries {
			modes[entry.artifact.Mode] = true
			if entry.artifact.Mode == state.ArtifactModeCopy {
				copyDigests[entry.artifact.Digest] = true
			}
		}
		if len(copyDigests) > 1 || (len(copyDigests) == 1 && len(modes) > 1) {
			conflicts[destination] = entries
		}
	}
	return conflicts
}

func (conflictingDestination) audit(env *Env) ([]Issue, error) {
	if !env.HasProject {
		return nil, nil
	}
	conflicts := conflictingDestinations(env.lockedProjectArtifacts())
	destinations := make([]string, 0, len(conflicts))
	for destination := range conflicts {
		destinations = append(destinations, destination)
	}
	sort.Strings(destinations)
	var issues []Issue
	for _, destination := range destinations {
		owners := map[string]bool{}
		var skills []string
		for _, entry := range conflicts[destination] {
			if !owners[entry.skill] {
				owners[entry.skill] = true
				skills = append(skills, entry.skill)
			}
		}
		sort.Strings(skills)
		issues = append(issues, Issue{
			Check:   conflictingDestination{}.id(),
			Scope:   scopeProject,
			Subject: destination,
			Detail:  fmt.Sprintf("managed by multiple skills with different content (%s)", strings.Join(skills, ", ")),
			Remedy:  "reconcile the catalogs so the skills share the file as Markdown sections, or one skill owns it",
		})
	}
	return issues, nil
}

func (conflictingDestination) fix(env *Env, issues []Issue) error {
	// Doctor cannot decide which skill owns the file. It can only repair a
	// conflict whose catalogs no longer conflict — e.g. after the skills
	// switched to markdown-section — by reinstalling every owner so the lock
	// migrates to the compatible entries.
	for _, issue := range issues {
		owners, resolvable := resolvableConflictOwners(env, issue.Subject)
		if !resolvable {
			continue
		}
		for _, owner := range owners {
			if err := reinstallProjectArtifacts(env, owner.skill, owner.global); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolvableConflictOwners reports whether every skill managing a conflicted
// destination currently resolves to compatible entries for it — no copy-mode
// entries with differing content and no copy mode sharing the file with
// other modes — and returns the owners to reinstall. Unresolvable skills
// (missing catalog caches) make the conflict unfixable here; the remedy text
// points at the catalogs.
func resolvableConflictOwners(env *Env, destination string) ([]lockedArtifact, bool) {
	var owners []lockedArtifact
	seen := map[string]bool{}
	modes := map[string]bool{}
	copyDigests := map[string]bool{}
	for _, locked := range env.lockedProjectArtifacts() {
		if locked.artifact.Destination != destination || seen[locked.skill] {
			continue
		}
		seen[locked.skill] = true
		owners = append(owners, locked)
		catalogName := locked.artifactCatalog(env)
		resolved, err := env.resolveSkill(env.ProjectManifest, locked.skill, catalogName)
		if err != nil {
			return nil, false
		}
		hooks := locked.artifactHooks(env)
		selected := installer.ProjectArtifacts(resolved, hooks)
		targets := locked.artifactTargets(env)
		for _, target := range append(targets, "all") {
			for _, artifact := range selected.Artifacts[target] {
				if filepath.Join(env.ProjectRoot, artifact.Destination) != destination {
					continue
				}
				modes[artifact.Mode] = true
				if artifact.Mode == state.ArtifactModeCopy {
					copyDigests[artifact.Digest] = true
				}
			}
		}
	}
	if len(copyDigests) > 1 || (len(copyDigests) == 1 && len(modes) > 1) {
		return nil, false
	}
	return owners, true
}

// artifactCatalog finds the catalog recorded for the skill that owns a lock
// artifact, in either lock.
func (a lockedArtifact) artifactCatalog(env *Env) string {
	if a.global {
		return env.GlobalLock.Projects[env.ProjectRoot][a.skill].Catalog
	}
	return env.ProjectLock.Skills[a.skill].Catalog
}

func (a lockedArtifact) artifactHooks(env *Env) bool {
	if a.global {
		return env.GlobalLock.Projects[env.ProjectRoot][a.skill].Hooks
	}
	return env.ProjectLock.Skills[a.skill].Hooks
}

func (a lockedArtifact) artifactTargets(env *Env) []string {
	if a.global {
		return env.GlobalLock.Projects[env.ProjectRoot][a.skill].Targets
	}
	return env.ProjectLock.Skills[a.skill].Targets
}

// reinstallProjectArtifacts reinstalls one skill's project artifacts from
// the catalog cache with force semantics, repairing missing destinations and
// locally modified managed content in one pass.
func reinstallProjectArtifacts(env *Env, skill string, global bool) error {
	var catalogName string
	var targetNames []string
	var previous []state.LockArtifact
	var hooks bool
	if global {
		entry, exists := env.GlobalLock.Projects[env.ProjectRoot][skill]
		if !exists {
			return fmt.Errorf("skill %q has no locked artifacts for this project", skill)
		}
		catalogName, targetNames, previous, hooks = entry.Catalog, entry.Targets, entry.Artifacts, entry.Hooks
	} else {
		entry, exists := env.ProjectLock.Skills[skill]
		if !exists {
			return fmt.Errorf("skill %q is not installed in the project lock", skill)
		}
		catalogName, targetNames, previous, hooks = entry.Catalog, entry.Targets, entry.Artifacts, entry.Hooks
	}
	resolved, err := env.resolveSkill(env.ProjectManifest, skill, catalogName)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", skill, err)
	}
	targets, err := installer.ResolveTargets(env.ProjectScope, targetNames, "")
	if err != nil {
		return err
	}
	selected := installer.ProjectArtifacts(resolved, hooks)
	artifacts, err := installer.InstallArtifacts(selected, targets, env.ProjectRoot, previous, true)
	if err != nil {
		return err
	}
	if global {
		projects := env.GlobalLock.Projects[env.ProjectRoot]
		entry := projects[skill]
		entry.Artifacts = artifacts
		projects[skill] = entry
		return env.saveGlobalLock()
	}
	entry := env.ProjectLock.Skills[skill]
	entry.Artifacts = artifacts
	env.ProjectLock.Skills[skill] = entry
	return env.saveProjectLock()
}

// reinstallArtifactSkills reinstalls every skill referenced by the given
// "skill<global?> " subject prefixes.
func reinstallArtifactSkills(env *Env, issues []Issue) error {
	type skillRef struct {
		skill  string
		global bool
	}
	repaired := map[skillRef]bool{}
	for _, issue := range issues {
		skill, global, ok := parseArtifactSubject(issue.Subject)
		if !ok || repaired[skillRef{skill, global}] {
			continue
		}
		repaired[skillRef{skill, global}] = true
		if err := reinstallProjectArtifacts(env, skill, global); err != nil {
			return err
		}
	}
	return nil
}

// artifactSubject formats the subject shared by the artifact checks:
// "skill:id (destination)" with a " (global lock)" suffix for bootstrap
// pointers. parseArtifactSubject reverses it.
func artifactSubject(locked lockedArtifact) string {
	subject := fmt.Sprintf("%s:%s (%s)", locked.skill, locked.artifact.ID, locked.artifact.Destination)
	if locked.global {
		subject += " (global lock)"
	}
	return subject
}

func parseArtifactSubject(subject string) (skill string, global bool, ok bool) {
	global = strings.HasSuffix(subject, " (global lock)")
	trimmed := strings.TrimSuffix(subject, " (global lock)")
	separator := strings.Index(trimmed, ":")
	if separator <= 0 {
		return "", false, false
	}
	return trimmed[:separator], global, true
}

// missingDestination: locked artifacts whose destination file is gone.
type missingDestination struct{}

func (missingDestination) id() string { return "missing-destination" }

func (missingDestination) audit(env *Env) ([]Issue, error) {
	if !env.HasProject {
		return nil, nil
	}
	var issues []Issue
	for _, locked := range env.lockedProjectArtifacts() {
		if _, err := os.Lstat(locked.artifact.Destination); os.IsNotExist(err) {
			issues = append(issues, Issue{
				Check:   missingDestination{}.id(),
				Scope:   scopeProject,
				Subject: artifactSubject(locked),
				Detail:  "managed destination is missing",
				Remedy:  "run repertoire doctor --fix",
			})
		}
	}
	return issues, nil
}

func (missingDestination) fix(env *Env, issues []Issue) error {
	return reinstallArtifactSkills(env, issues)
}

// modifiedContent: managed content whose on-disk state no longer matches the
// lock (local edits, partial writes, malformed markers).
type modifiedContent struct{}

func (modifiedContent) id() string { return "modified-managed-content" }

func (modifiedContent) audit(env *Env) ([]Issue, error) {
	if !env.HasProject {
		return nil, nil
	}
	conflicts := conflictingDestinations(env.lockedProjectArtifacts())
	var issues []Issue
	for _, locked := range env.lockedProjectArtifacts() {
		if _, conflicted := conflicts[locked.artifact.Destination]; conflicted {
			continue // conflicting-destination explains these
		}
		if _, err := os.Lstat(locked.artifact.Destination); os.IsNotExist(err) {
			continue // missingDestination owns these
		}
		detail := ""
		switch locked.artifact.Mode {
		case state.ArtifactModeMarkdownSection:
			digest, found, err := installer.MarkdownSectionDigest(locked.artifact.Destination, locked.artifact.Marker)
			switch {
			case err != nil:
				detail = "managed Markdown markers are malformed"
			case !found:
				detail = "managed Markdown section is missing from the destination"
			case digest != locked.artifact.Digest:
				detail = "managed Markdown section is locally modified"
			}
		case state.ArtifactModeCopy:
			digest, err := installer.DigestFile(locked.artifact.Destination)
			if err != nil {
				return nil, err
			}
			if digest != locked.artifact.Digest {
				detail = "managed file is locally modified"
			}
		case state.ArtifactModeJSONMerge:
			matches, err := installer.VerifyJSONArtifact(locked.artifact.Destination, locked.artifact)
			if err != nil {
				detail = "managed JSON destination is unreadable"
			} else if !matches {
				detail = "managed JSON entries are locally modified"
			}
		}
		if detail != "" {
			issues = append(issues, Issue{
				Check:   modifiedContent{}.id(),
				Scope:   scopeProject,
				Subject: artifactSubject(locked),
				Detail:  detail,
				Remedy:  "run repertoire doctor --fix",
			})
		}
	}
	return issues, nil
}

func (modifiedContent) fix(env *Env, issues []Issue) error {
	return reinstallArtifactSkills(env, issues)
}

// duplicateSections: identical managed sections repeated under several
// per-target markers in one file — the layout produced before the installer
// deduplicated per-target instructions. The fix collapses each group to the
// single shared-marker section the current installer would write.
type duplicateSections struct{}

func (duplicateSections) id() string { return "duplicate-sections" }

func (duplicateSections) audit(env *Env) ([]Issue, error) {
	if !env.HasProject {
		return nil, nil
	}
	sections, err := env.scanSections()
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, group := range duplicateGroups(sections) {
		if !groupHasLockedMember(env, group) {
			continue // fully orphaned groups belong to orphanedMarkers
		}
		first := group[0]
		issues = append(issues, Issue{
			Check:   duplicateSections{}.id(),
			Scope:   scopeProject,
			Subject: fmt.Sprintf("%s:%s (%s)", first.skill, first.id, first.destination),
			Detail:  fmt.Sprintf("%d identical managed sections under per-target markers", len(group)),
			Remedy:  "run repertoire doctor --fix",
		})
	}
	return issues, nil
}

func groupHasLockedMember(env *Env, group []fileSection) bool {
	locked := env.lockedMarkers()
	for _, member := range group {
		if locked[member.section.Marker] {
			return true
		}
	}
	return false
}

func (duplicateSections) fix(env *Env, issues []Issue) error {
	sections, err := env.scanSections()
	if err != nil {
		return err
	}
	for _, group := range duplicateGroups(sections) {
		if err := env.collapseGroup(group); err != nil {
			return err
		}
	}
	return nil
}

// collapseGroup rewrites one duplicate group as a single shared-marker
// section and rewrites the lock entries to match. Only members whose locked
// content is unmodified take part; a locally modified member is left for the
// modified-managed-content check, and a group with fewer than two eligible
// members is left alone (unlocked leftovers belong to orphaned-markers).
func (env *Env) collapseGroup(group []fileSection) error {
	type member struct {
		entry state.LockArtifact
		fileSection
		global bool
	}
	var eligible []member
	for _, candidate := range group {
		skill, entry, global, found := env.lockEntryFor(candidate.section.Marker)
		if !found || skill == "" {
			continue
		}
		digest, digestFound, err := installer.MarkdownSectionDigest(candidate.destination, candidate.section.Marker)
		if err != nil || !digestFound || digest != entry.Digest {
			continue
		}
		eligible = append(eligible, member{fileSection: candidate, entry: entry, global: global})
	}
	if len(eligible) < 2 {
		return nil
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].section.Start < eligible[j].section.Start })

	first := eligible[0]
	content, err := os.ReadFile(first.destination)
	if err != nil {
		return err
	}
	updated := content
	for index := len(eligible) - 1; index >= 1; index-- {
		updated = installer.RemoveMarkedSection(updated, eligible[index].section)
	}
	marker := "repertoire:" + first.skill + ":all:" + first.id
	block := installer.RenderMarkedSection(marker, first.section.Content)
	replacement := append([]byte(nil), updated[:first.section.Start]...)
	replacement = append(replacement, block...)
	replacement = append(replacement, updated[first.section.End:]...)
	if writeErr := state.WriteFileAtomic(first.destination, replacement, 0o644); writeErr != nil {
		return writeErr
	}
	digest, found, err := installer.MarkdownSectionDigest(first.destination, marker)
	if err != nil || !found {
		return fmt.Errorf("verify collapsed section %q: %w", marker, err)
	}

	separator := "\n"
	if first.section.Start == 0 {
		separator = ""
	}
	collapsed := state.LockArtifact{
		ID: first.id, Target: "all", Destination: first.destination,
		Mode: state.ArtifactModeMarkdownSection, Marker: marker, Digest: digest,
		MarkdownSeparator: separator,
	}
	for _, member := range eligible {
		collapsed.Created = collapsed.Created || member.entry.Created
	}
	markers := map[string]bool{}
	for _, member := range eligible {
		markers[member.section.Marker] = true
	}
	env.removeLockedSections(markers)
	env.addLockedSection(first.skill, first.global, collapsed)
	if err := env.saveProjectLock(); err != nil {
		return err
	}
	return env.saveGlobalLock()
}

func (env *Env) removeLockedSections(markers map[string]bool) {
	for name, skill := range env.ProjectLock.Skills {
		skill.Artifacts = filterArtifacts(skill.Artifacts, markers)
		env.ProjectLock.Skills[name] = skill
	}
	projects := env.GlobalLock.Projects[env.ProjectRoot]
	for name, pointers := range projects {
		pointers.Artifacts = filterArtifacts(pointers.Artifacts, markers)
		projects[name] = pointers
	}
}

func filterArtifacts(artifacts []state.LockArtifact, markers map[string]bool) []state.LockArtifact {
	kept := artifacts[:0]
	for _, artifact := range artifacts {
		if !markers[artifact.Marker] {
			kept = append(kept, artifact)
		}
	}
	return kept
}

func (env *Env) addLockedSection(skill string, global bool, artifact state.LockArtifact) {
	if global {
		projects := env.GlobalLock.Projects[env.ProjectRoot]
		entry := projects[skill]
		entry.Artifacts = append(entry.Artifacts, artifact)
		projects[skill] = entry
		return
	}
	entry := env.ProjectLock.Skills[skill]
	entry.Artifacts = append(entry.Artifacts, artifact)
	env.ProjectLock.Skills[skill] = entry
}

// orphanedMarkers: managed sections no lock entry claims. The marker itself
// is a management claim, so --fix removes every orphan.
type orphanedMarkers struct{}

func (orphanedMarkers) id() string { return "orphaned-markers" }

func (orphanedMarkers) audit(env *Env) ([]Issue, error) {
	if !env.HasProject {
		return nil, nil
	}
	sections, err := env.scanSections()
	if err != nil {
		return nil, err
	}
	locked := env.lockedMarkers()
	duplicates := map[string]bool{}
	for _, group := range duplicateGroups(sections) {
		if !groupHasLockedMember(env, group) {
			continue // fully orphaned groups are reported block by block below
		}
		for _, member := range group {
			duplicates[member.section.Marker] = true
		}
	}
	var issues []Issue
	for _, candidate := range sections {
		if locked[candidate.section.Marker] || duplicates[candidate.section.Marker] {
			continue
		}
		issues = append(issues, Issue{
			Check:   orphanedMarkers{}.id(),
			Scope:   scopeProject,
			Subject: fmt.Sprintf("%s:%s (%s)", candidate.skill, candidate.id, candidate.destination),
			Detail:  fmt.Sprintf("managed section with marker %q has no lock entry", candidate.section.Marker),
			Remedy:  "run repertoire doctor --fix",
		})
	}
	return issues, nil
}

func (orphanedMarkers) fix(env *Env, issues []Issue) error {
	sections, err := env.scanSections()
	if err != nil {
		return err
	}
	locked := env.lockedMarkers()
	for _, candidate := range sections {
		if locked[candidate.section.Marker] {
			continue
		}
		if err := installer.RemoveMarkedSectionFromFile(candidate.destination, candidate.section.Marker); err != nil {
			return err
		}
	}
	return nil
}

// manifestDrift: skills declared in repertoire.yaml whose lock state does
// not match. Report-only here; the CLI runs the sync/install path on --fix.
type manifestDrift struct{}

func (manifestDrift) id() string { return "manifest-drift" }

func (manifestDrift) audit(env *Env) ([]Issue, error) {
	if !env.HasProject {
		return nil, nil
	}
	var issues []Issue
	names := make([]string, 0, len(env.ProjectManifest.Skills))
	for name := range env.ProjectManifest.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		declaration := env.ProjectManifest.Skills[name]
		detail := ""
		if declaration.Scope == state.BootstrapScopeGlobal {
			entry, installed := env.GlobalLock.Skills[name]
			switch {
			case !installed:
				detail = "declared with global scope but not installed"
			case expectsProjectPointers(env, name, entry) && !hasProjectPointers(env, name):
				detail = "installed globally but project pointers are missing"
			}
		} else {
			entry, installed := env.ProjectLock.Skills[name]
			switch {
			case !installed:
				detail = "declared but not installed"
			case len(declaration.Targets) > 0 && !sameStringSet(entry.Targets, declaration.Targets):
				detail = fmt.Sprintf("locked targets %s differ from declared targets %s",
					strings.Join(entry.Targets, ", "), strings.Join(declaration.Targets, ", "))
			}
		}
		if detail != "" {
			issues = append(issues, Issue{
				Check:   manifestDrift{}.id(),
				Scope:   scopeProject,
				Subject: name,
				Detail:  detail,
				Remedy:  "run repertoire sync",
			})
		}
	}
	requirements := make([]string, 0, len(env.ProjectManifest.Requirements))
	for name := range env.ProjectManifest.Requirements {
		requirements = append(requirements, name)
	}
	sort.Strings(requirements)
	for _, name := range requirements {
		if lockHasSkill(env.ProjectLock, name) || lockHasSkill(env.GlobalLock, name) {
			continue
		}
		issues = append(issues, Issue{
			Check:   manifestDrift{}.id(),
			Scope:   scopeProject,
			Subject: name,
			Detail:  "required but not installed",
			Remedy:  "run repertoire install",
		})
	}
	return issues, nil
}

func (manifestDrift) fix(_ *Env, _ []Issue) error {
	// The CLI reconciles drift through the sync/install path; doctor itself
	// only reports it.
	return nil
}

func lockHasSkill(lock state.Lock, name string) bool {
	for installed := range lock.Skills {
		if installed == name || skillLeaf(installed) == skillLeaf(name) {
			return true
		}
	}
	return false
}

func hasProjectPointers(env *Env, name string) bool {
	_, exists := env.GlobalLock.Projects[env.ProjectRoot][name]
	return exists
}

// expectsProjectPointers reports whether a global-scope bootstrap skill
// should have per-project artifact pointers in the global lock. Skills
// without project instructions or hooks never gain pointers, so their
// absence is healthy, not drift. Unresolvable skills are given the benefit
// of the doubt.
func expectsProjectPointers(env *Env, name string, entry state.LockSkill) bool {
	resolved, err := env.resolveSkill(env.ProjectManifest, name, entry.Catalog)
	if err != nil {
		return false
	}
	pools := []map[string][]installer.ResolvedArtifact{resolved.Instructions}
	if entry.Hooks {
		pools = append(pools, resolved.Artifacts)
	}
	for _, pool := range pools {
		if len(pool["all"]) > 0 {
			return true
		}
		for _, target := range entry.Targets {
			if len(pool[target]) > 0 {
				return true
			}
		}
	}
	return false
}

// staleProjectEntries: global-lock project pointers for directories that no
// longer exist.
type staleProjectEntries struct{}

func (staleProjectEntries) id() string { return "stale-project-entries" }

func (staleProjectEntries) audit(env *Env) ([]Issue, error) {
	var issues []Issue
	roots := make([]string, 0, len(env.GlobalLock.Projects))
	for root := range env.GlobalLock.Projects {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			issues = append(issues, Issue{
				Check:   staleProjectEntries{}.id(),
				Scope:   scopeGlobal,
				Subject: root,
				Detail:  "lock holds project pointers for a directory that no longer exists",
				Remedy:  "run repertoire doctor --fix",
			})
		}
	}
	return issues, nil
}

func (staleProjectEntries) fix(env *Env, issues []Issue) error {
	for _, issue := range issues {
		delete(env.GlobalLock.Projects, issue.Subject)
	}
	return env.saveGlobalLock()
}

// globalSkillHealth: globally installed skills whose install locations are
// missing or whose content digest no longer matches the lock.
type globalSkillHealth struct{}

func (globalSkillHealth) id() string { return "global-skill-health" }

func (globalSkillHealth) audit(env *Env) ([]Issue, error) {
	var issues []Issue
	names := make([]string, 0, len(env.GlobalLock.Skills))
	for name := range env.GlobalLock.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := env.GlobalLock.Skills[name]
		expected := map[string]bool{entry.Digest: true}
		for _, digest := range entry.TargetDigests {
			expected[digest] = true
		}
		for _, location := range entry.Locations {
			detail := ""
			if _, err := os.Lstat(location); os.IsNotExist(err) {
				detail = "installed location is missing"
			} else {
				digest, err := installer.Digest(location)
				if err != nil {
					return nil, err
				}
				if !expected[digest] {
					detail = "installed content is locally modified or partial"
				}
			}
			if detail != "" {
				issues = append(issues, Issue{
					Check:   globalSkillHealth{}.id(),
					Scope:   scopeGlobal,
					Subject: fmt.Sprintf("%s (%s)", name, location),
					Detail:  detail,
					Remedy:  "run repertoire doctor --fix",
				})
			}
		}
	}
	return issues, nil
}

func (globalSkillHealth) fix(env *Env, issues []Issue) error {
	repaired := map[string]bool{}
	for _, issue := range issues {
		name := issue.Subject
		if separator := strings.Index(name, " ("); separator > 0 {
			name = name[:separator]
		}
		if repaired[name] {
			continue
		}
		repaired[name] = true
		if err := reinstallGlobalSkill(env, name); err != nil {
			return fmt.Errorf("reinstall %q: %w (run repertoire update %s)", name, err, name)
		}
	}
	return nil
}

func reinstallGlobalSkill(env *Env, name string) error {
	entry, exists := env.GlobalLock.Skills[name]
	if !exists {
		return fmt.Errorf("skill %q is not installed", name)
	}
	resolved, err := env.resolveSkill(env.GlobalManifest, name, entry.Catalog)
	if err != nil {
		return err
	}
	targets, err := installer.ResolveTargets(env.GlobalScope, entry.Targets, "")
	if err != nil {
		return err
	}
	locations, digests, err := installer.SkillWithDigests(resolved, targets, &entry, true)
	if err != nil {
		return err
	}
	entry.Locations = locations
	entry.TargetDigests = digests
	entry.Digest = resolved.Digest
	env.GlobalLock.Skills[name] = entry
	return env.saveGlobalLock()
}
