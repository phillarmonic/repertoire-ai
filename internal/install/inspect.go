package install

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/phillarmonic/repertoire-ai/internal/state"
)

// MarkedSection is one repertoire-managed Markdown block found in a file.
// Start and End are byte offsets into the scanned content; End points just
// past the newline that terminates the end marker.
type MarkedSection struct {
	Marker  string
	Content []byte
	Start   int
	End     int
}

// ParseMarker splits a managed-section marker prefix into its skill, target,
// and artifact id parts. It reports false for markers ScanMarkedSections
// would never produce ("repertoire:<skill>:<target>:<id>").
func ParseMarker(marker string) (skill, target, id string, ok bool) {
	parts := strings.Split(marker, ":")
	if len(parts) != 4 || parts[0] != "repertoire" {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

// ScanMarkedSections finds every well-formed repertoire-managed section in
// content. Malformed markers (a start marker without its matching end
// marker, or vice versa) are skipped; callers verify specific locked
// sections with MarkdownSectionDigest, which surfaces malformed markers as
// errors.
func ScanMarkedSections(content []byte) []MarkedSection {
	const commentStart = "<!-- repertoire:"
	var sections []MarkedSection
	offset := 0
	for {
		relative := bytes.Index(content[offset:], []byte(commentStart))
		if relative < 0 {
			return sections
		}
		start := offset + relative
		markerEnd := bytes.Index(content[start:], []byte(":start -->"))
		if markerEnd < 0 {
			return sections
		}
		marker := string(content[start+len("<!-- ") : start+markerEnd])
		bodyStart := start + markerEnd + len(":start -->")
		endMarker := []byte("<!-- " + marker + ":end -->")
		endRelative := bytes.Index(content[bodyStart:], endMarker)
		if endRelative < 0 {
			// Unterminated section; skip the start marker and keep scanning.
			offset = bodyStart
			continue
		}
		end := bodyStart + endRelative + len(endMarker)
		if end < len(content) && content[end] == '\n' {
			end++
		}
		sections = append(sections, MarkedSection{
			Marker:  marker,
			Content: bytes.TrimSpace(content[bodyStart : bodyStart+endRelative]),
			Start:   start,
			End:     end,
		})
		offset = end
	}
}

// RenderMarkedSection renders one managed section with the given marker
// prefix, matching the layout InstallArtifacts writes.
func RenderMarkedSection(markerPrefix string, content []byte) []byte {
	start, end := artifactMarkersFromLock(state.LockArtifact{Marker: markerPrefix})
	return renderMarkedSection(start, end, content)
}

// MarkdownSectionDigest recomputes the digest of the managed section
// identified by markerPrefix in the file at destination.
func MarkdownSectionDigest(destination, markerPrefix string) (digest string, found bool, err error) {
	content, err := readOptionalFile(destination)
	if err != nil || content == nil {
		return "", false, err
	}
	start, end := artifactMarkersFromLock(state.LockArtifact{Marker: markerPrefix})
	current, found, err := markedSection(content, start, end)
	if err != nil || !found {
		return "", found, err
	}
	return digestBytes(current), true, nil
}

// RemoveMarkedSectionFromFile removes the managed section identified by
// markerPrefix from the file at destination, along with the separator line
// that precedes it when one is present. Unlike RemoveArtifacts it never
// deletes the file itself, so inspection tooling can strip orphaned or
// duplicated blocks without destroying user-facing files.
func RemoveMarkedSectionFromFile(destination, markerPrefix string) error {
	content, err := readOptionalFile(destination)
	if err != nil || content == nil {
		return err
	}
	for _, section := range ScanMarkedSections(content) {
		if section.Marker != markerPrefix {
			continue
		}
		updated := RemoveMarkedSection(content, section)
		return state.WriteFileAtomic(destination, updated, 0o644)
	}
	return fmt.Errorf("managed section %q not found in %s", markerPrefix, destination)
}

// RemoveMarkedSection removes section from content, including one preceding
// separator newline when the section does not start the file.
func RemoveMarkedSection(content []byte, section MarkedSection) []byte {
	start := section.Start
	if start > 0 && content[start-1] == '\n' {
		start--
	}
	updated := append([]byte(nil), content[:start]...)
	updated = append(updated, content[section.End:]...)
	if len(bytes.TrimSpace(updated)) == 0 {
		return bytes.TrimRight(updated, "\n")
	}
	return updated
}

// VerifyJSONArtifact reports whether the managed JSON entries recorded in
// the lock are still present, unmodified, in the file at destination.
func VerifyJSONArtifact(destination string, artifact state.LockArtifact) (bool, error) {
	existingBytes, err := readOptionalFile(destination)
	if err != nil {
		return false, err
	}
	if existingBytes == nil {
		return false, errors.New("destination is missing")
	}
	existing, _, err := decodeJSONObject(existingBytes)
	if err != nil {
		return false, err
	}
	fragment, _, err := decodeJSONObject(artifact.ManagedJSON)
	if err != nil {
		return false, err
	}
	return jsonFragmentMatches(existing, fragment), nil
}
