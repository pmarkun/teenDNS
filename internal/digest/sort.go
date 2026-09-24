package digest

import "sort"

// sortCategoryDigests makes report order deterministic: categories
// alphabetically, domains within a category alphabetically.
func sortCategoryDigests(digests []CategoryDigest) {
	sort.Slice(digests, func(i, j int) bool {
		return digests[i].Category < digests[j].Category
	})
	for index := range digests {
		domains := digests[index].Domains
		sort.Slice(domains, func(i, j int) bool {
			return domains[i].Domain < domains[j].Domain
		})
	}
}
