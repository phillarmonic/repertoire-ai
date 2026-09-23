package install

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/phillarmonic/repertoire-ai/internal/catalog"
)

// VerifyDigestSignature accepts a trusted catalog skill only when
// DigestSignatureFile is a valid detached signature over the exact hex text
// of Digest. Either declared key may sign. The returned fingerprint is that
// key. A registration with no trust block skips the check.
func VerifyDigestSignature(skillPath string, source catalog.Source) (string, error) {
	if source.Registration.Trust == nil {
		return "", nil
	}
	digest, err := Digest(skillPath)
	if err != nil {
		return "", fmt.Errorf("catalog %q skill %s: %w", source.Name, skillPath, err)
	}
	signaturePath := filepath.Join(skillPath, DigestSignatureFile)
	return catalog.VerifyDetachedSignature(source, skillPath, signaturePath, []byte(digest))
}

// VerifyTrustedDigest checks the resolved skill, and each variant, before a
// trusted catalog skill is written into the lock. The returned fingerprint
// signed the skill digest. A registration with no trust block skips the
// check. A failure names the catalog, skill, and commit.
func VerifyTrustedDigest(resolved ResolvedSkill) (string, error) {
	source := resolved.Catalog.Source
	if source.Registration.Trust == nil {
		return "", nil
	}
	fingerprint, err := verifyOneDigest(resolved, "", resolved.Root, source)
	if err != nil {
		return "", err
	}
	for target, variant := range resolved.Variants {
		if _, err := verifyOneDigest(resolved, target, variant.Root, source); err != nil {
			return "", err
		}
	}
	return fingerprint, nil
}

func verifyOneDigest(resolved ResolvedSkill, variant, skillPath string, source catalog.Source) (string, error) {
	fingerprint, err := VerifyDigestSignature(skillPath, source)
	if err == nil {
		return fingerprint, nil
	}
	skill := resolved.Name
	if variant != "" {
		skill = resolved.Name + " (" + variant + ")"
	}
	detail := err.Error()
	if failure, ok := errors.AsType[*catalog.TrustFailure](err); ok {
		detail = failure.Detail
	}
	return "", fmt.Errorf("catalog %q skill %q commit %s: digest signature failed: %s", resolved.Catalog.Name, skill, resolved.Catalog.Commit, detail)
}
