package cli

import "strings"

// splitNameList expands positional arguments into individual names. Each
// argument may itself be a comma-separated list, so `alpha,beta` and
// `alpha beta` both select alpha and beta. Elements are trimmed, empty
// elements are dropped, and the original order is kept with duplicates
// removed.
func splitNameList(args []string) []string {
	names := make([]string, 0, len(args))
	for _, arg := range args {
		for part := range strings.SplitSeq(arg, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				names = append(names, part)
			}
		}
	}
	return dedupeNames(names)
}
