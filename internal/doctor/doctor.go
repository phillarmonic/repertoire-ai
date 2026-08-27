// Package doctor diagnoses and repairs repertoire-managed installations:
// missing or locally modified managed files, orphaned or duplicated managed
// Markdown sections, manifest drift, stale lock entries, and broken skill
// installs. Detection and repair share one audit path so `repertoire doctor`
// and `repertoire doctor --fix` never disagree about what is wrong.
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
	installer "github.com/phillarmonic/repertoire-ai/internal/install"
	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// Issue is one diagnosed problem. Remedy is the human-facing suggestion
// shown in report mode; Fixed marks issues repaired during a --fix run.
type Issue struct {
	Check   string `json:"check"`
	Scope   string `json:"scope"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
	Remedy  string `json:"remedy"`
	Fixed   bool   `json:"fixed,omitempty"`
}

// Result carries the issues found by Run. In report mode Issues holds every
// finding; after a fix run, Fixed holds repaired issues and Issues holds the
// residual ones.
type Result struct {
	Issues []Issue `json:"issues"`
	Fixed  []Issue `json:"fixed,omitempty"`
}

// Env is the state doctor inspects. HasProject is false outside a Git
// worktree, in which case project checks are skipped. ResolveSkill resolves
// a skill from the catalog cache; nil uses installer.Resolve.
type Env struct {
	Manager         *catalog.Manager
	ResolveSkill    func(manifest state.Manifest, name, catalogName string) (installer.ResolvedSkill, error)
	ProjectLock     state.Lock
	GlobalLock      state.Lock
	ProjectRoot     string
	ProjectManifest state.Manifest
	GlobalManifest  state.Manifest
	ProjectScope    state.Scope
	GlobalScope     state.Scope
	HasProject      bool
}

const (
	scopeProject = "project"
	scopeGlobal  = "global"
)

type check interface {
	id() string
	audit(env *Env) ([]Issue, error)
	fix(env *Env, issues []Issue) error
}

// checks run in this order; conflicting-destination precedes the repair
// checks so structural conflicts are reported instead of flapping per-entry
// modification reports, and duplicate-sections precedes orphaned-markers so
// orphans never swallow blocks a collapse owns.
var checks = []check{
	conflictingDestination{},
	missingDestination{},
	modifiedContent{},
	duplicateSections{},
	orphanedMarkers{},
	manifestDrift{},
	staleProjectEntries{},
	globalSkillHealth{},
}

// Run audits the environment and, when fix is true, repairs what it can.
// After a fix run each check is re-audited so Issues only holds residual
// problems.
func Run(env *Env, fix bool) (Result, error) {
	var result Result
	for _, current := range checks {
		issues, err := current.audit(env)
		if err != nil {
			return result, fmt.Errorf("check %s: %w", current.id(), err)
		}
		if !fix || len(issues) == 0 {
			result.Issues = append(result.Issues, issues...)
			continue
		}
		if fixErr := current.fix(env, issues); fixErr != nil {
			return result, fmt.Errorf("fix %s: %w", current.id(), fixErr)
		}
		residual, err := current.audit(env)
		if err != nil {
			return result, fmt.Errorf("re-audit %s: %w", current.id(), err)
		}
		result.Fixed = append(result.Fixed, repairedIssues(issues, residual)...)
		result.Issues = append(result.Issues, residual...)
	}
	sortIssues(result.Issues)
	sortIssues(result.Fixed)
	return result, nil
}

func repairedIssues(before, residual []Issue) []Issue {
	remaining := map[string]int{}
	for _, issue := range residual {
		remaining[issueKey(issue)]++
	}
	var repaired []Issue
	for _, issue := range before {
		key := issueKey(issue)
		if remaining[key] > 0 {
			remaining[key]--
			continue
		}
		issue.Fixed = true
		repaired = append(repaired, issue)
	}
	return repaired
}

func issueKey(issue Issue) string {
	return issue.Check + "\x00" + issue.Scope + "\x00" + issue.Subject + "\x00" + issue.Detail
}

func sortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Check != issues[j].Check {
			return issues[i].Check < issues[j].Check
		}
		if issues[i].Scope != issues[j].Scope {
			return issues[i].Scope < issues[j].Scope
		}
		return issues[i].Subject < issues[j].Subject
	})
}

func (env *Env) resolveSkill(manifest state.Manifest, name, catalogName string) (installer.ResolvedSkill, error) {
	if env.ResolveSkill != nil {
		return env.ResolveSkill(manifest, name, catalogName)
	}
	return installer.Resolve(env.Manager, manifest, name, catalogName, false)
}

func (env *Env) saveProjectLock() error {
	return state.SaveLock(env.ProjectScope.LockPath, env.ProjectLock)
}

func (env *Env) saveGlobalLock() error {
	return state.SaveLock(env.GlobalScope.LockPath, env.GlobalLock)
}

// lockedArtifact pairs a lock entry with the skill and lock that own it.
type lockedArtifact struct {
	skill    string
	artifact state.LockArtifact
	global   bool
}

// lockedProjectArtifacts returns every artifact locked against the current
// project, from the project lock and from the global lock's per-project
// bootstrap pointers.
func (env *Env) lockedProjectArtifacts() []lockedArtifact {
	var locked []lockedArtifact
	for name, skill := range env.ProjectLock.Skills {
		for _, artifact := range skill.Artifacts {
			locked = append(locked, lockedArtifact{skill: name, artifact: artifact})
		}
	}
	for name, pointers := range env.GlobalLock.Projects[env.ProjectRoot] {
		for _, artifact := range pointers.Artifacts {
			locked = append(locked, lockedArtifact{skill: name, global: true, artifact: artifact})
		}
	}
	return locked
}

func (env *Env) lockedMarkers() map[string]bool {
	markers := map[string]bool{}
	for _, locked := range env.lockedProjectArtifacts() {
		if locked.artifact.Marker != "" {
			markers[locked.artifact.Marker] = true
		}
	}
	return markers
}

// scanDestinations is the set of files that may carry managed sections: lock
// destinations, destinations of skills declared by the project manifest, and
// the well-known instruction files.
func (env *Env) scanDestinations() []string {
	destinations := map[string]bool{}
	for _, locked := range env.lockedProjectArtifacts() {
		destinations[locked.artifact.Destination] = true
	}
	declared := make([]string, 0, len(env.ProjectManifest.Skills)+len(env.ProjectManifest.Requirements))
	for name := range env.ProjectManifest.Skills {
		declared = append(declared, name)
	}
	for name := range env.ProjectManifest.Requirements {
		declared = append(declared, name)
	}
	for _, name := range declared {
		resolved, err := env.resolveSkill(env.ProjectManifest, name, "")
		if err != nil {
			continue
		}
		selected := installer.ProjectArtifacts(resolved, true)
		for _, artifacts := range selected.Artifacts {
			for _, artifact := range artifacts {
				if artifact.Mode == state.ArtifactModeMarkdownSection {
					destinations[filepath.Join(env.ProjectRoot, artifact.Destination)] = true
				}
			}
		}
	}
	destinations[filepath.Join(env.ProjectRoot, "AGENTS.md")] = true
	destinations[filepath.Join(env.ProjectRoot, "CLAUDE.md")] = true
	result := make([]string, 0, len(destinations))
	for destination := range destinations {
		result = append(result, destination)
	}
	sort.Strings(result)
	return result
}

type fileSection struct {
	destination string
	skill       string
	target      string
	id          string
	section     installer.MarkedSection
}

// scanSections reads every scan destination and returns its managed
// sections with parsed markers. Sections with unparseable markers are
// ignored: doctor only reasons about well-formed repertoire sections.
func (env *Env) scanSections() ([]fileSection, error) {
	var sections []fileSection
	for _, destination := range env.scanDestinations() {
		// #nosec G304 -- destination comes from the resolved lock entry
		content, err := os.ReadFile(destination)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, section := range installer.ScanMarkedSections(content) {
			skill, target, id, ok := installer.ParseMarker(section.Marker)
			if !ok {
				continue
			}
			sections = append(sections, fileSection{
				destination: destination,
				section:     section,
				skill:       skill,
				target:      target,
				id:          id,
			})
		}
	}
	return sections, nil
}

// duplicateGroups groups scanned sections by (destination, skill, id) and
// returns the groups that hold at least two distinct markers with identical
// content — the bloat left behind by per-target instruction declarations.
func duplicateGroups(sections []fileSection) [][]fileSection {
	type groupKey struct {
		destination string
		skill       string
		id          string
	}
	byKey := map[string][]fileSection{}
	var order []string
	for _, section := range sections {
		key := groupKey{destination: section.destination, skill: section.skill, id: section.id}
		flat := key.destination + "\x00" + key.skill + "\x00" + key.id
		if _, seen := byKey[flat]; !seen {
			order = append(order, flat)
		}
		byKey[flat] = append(byKey[flat], section)
	}
	var groups [][]fileSection
	for _, flat := range order {
		members := byKey[flat]
		markers := map[string]bool{}
		identical := true
		for i, member := range members {
			markers[member.section.Marker] = true
			if i > 0 && string(member.section.Content) != string(members[0].section.Content) {
				identical = false
			}
		}
		if len(markers) >= 2 && identical {
			groups = append(groups, members)
		}
	}
	return groups
}

// lockEntryFor finds the lock artifact owning a marker, preferring the
// project lock over the global lock's per-project pointers.
func (env *Env) lockEntryFor(marker string) (skill string, entry state.LockArtifact, global bool, found bool) {
	for _, locked := range env.lockedProjectArtifacts() {
		if locked.artifact.Marker == marker {
			return locked.skill, locked.artifact, locked.global, true
		}
	}
	return "", state.LockArtifact{}, false, false
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := map[string]int{}
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func skillLeaf(name string) string {
	for index := len(name) - 1; index >= 0; index-- {
		if name[index] == '/' {
			return name[index+1:]
		}
	}
	return name
}
