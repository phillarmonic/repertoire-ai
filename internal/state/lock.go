package state

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	LockOriginDeclared  = "declared"
	LockOriginBootstrap = "bootstrap"
	LockOriginAdHoc     = "ad-hoc"
)

type Lock struct {
	Schema int                  `json:"schema"`
	Skills map[string]LockSkill `json:"skills"`
}

type LockSkill struct {
	Catalog       string            `json:"catalog"`
	Source        string            `json:"source"`
	Ref           string            `json:"ref,omitempty"`
	Commit        string            `json:"commit"`
	Digest        string            `json:"digest"`
	TargetDigests map[string]string `json:"target_digests,omitempty"`
	Targets       []string          `json:"targets"`
	Locations     []string          `json:"locations"`
	Artifacts     []LockArtifact    `json:"artifacts,omitempty"`
	Declared      bool              `json:"declared"`
	Origin        string            `json:"origin,omitempty"`
}

type LockArtifact struct {
	ID          string          `json:"id"`
	Target      string          `json:"target"`
	Destination string          `json:"destination"`
	Mode        string          `json:"mode"`
	Marker      string          `json:"marker,omitempty"`
	Digest      string          `json:"digest"`
	Created     bool            `json:"created,omitempty"`
	ManagedJSON json.RawMessage `json:"managed_json,omitempty"`
}

func NewLock() Lock {
	return Lock{Schema: SchemaVersion, Skills: map[string]LockSkill{}}
}

func LoadLock(path string) (Lock, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewLock(), nil
	}
	if err != nil {
		return Lock{}, fmt.Errorf("read lock: %w", err)
	}
	lock := NewLock()
	if err := json.Unmarshal(content, &lock); err != nil {
		return Lock{}, fmt.Errorf("decode lock: %w", err)
	}
	if lock.Schema != SchemaVersion {
		return Lock{}, fmt.Errorf("unsupported lock schema %d", lock.Schema)
	}
	for name, skill := range lock.Skills {
		if skill.Origin == "" {
			skill.Origin = skill.EffectiveOrigin()
		} else if !validLockOrigin(skill.Origin) {
			return Lock{}, fmt.Errorf("skill %q has unknown lock origin %q", name, skill.Origin)
		}
		lock.Skills[name] = skill
	}
	return lock, nil
}

func (l Lock) Marshal() ([]byte, error) {
	if l.Schema != SchemaVersion {
		return nil, fmt.Errorf("unsupported lock schema %d", l.Schema)
	}
	normalized := NewLock()
	for name, skill := range l.Skills {
		if skill.Origin == "" {
			skill.Origin = skill.EffectiveOrigin()
		}
		if !validLockOrigin(skill.Origin) {
			return nil, fmt.Errorf("skill %q has unknown lock origin %q", name, skill.Origin)
		}
		normalized.Skills[name] = skill
	}
	content, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode lock: %w", err)
	}
	return append(content, '\n'), nil
}

func (s LockSkill) EffectiveOrigin() string {
	if s.Origin != "" {
		return s.Origin
	}
	if s.Declared {
		return LockOriginDeclared
	}
	return LockOriginAdHoc
}

func validLockOrigin(origin string) bool {
	return origin == LockOriginDeclared || origin == LockOriginBootstrap || origin == LockOriginAdHoc
}
