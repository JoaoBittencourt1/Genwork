package githubsync

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	maxFinalBranches    = 20
	maxBranchNameLength = 255
	fieldOpenedColumn   = "openedColumnToken"
	fieldReviewColumn   = "reviewRequestedColumnToken"
	fieldMergedColumn   = "mergedColumnToken"
	fieldFinalBranches  = "finalBranches"
)

type UpdateBoardGithubSettingsRequest struct {
	OpenedColumnToken          httpapi.PatchField[string]   `json:"openedColumnToken"`
	ReviewRequestedColumnToken httpapi.PatchField[string]   `json:"reviewRequestedColumnToken"`
	MergedColumnToken          httpapi.PatchField[string]   `json:"mergedColumnToken"`
	FinalBranches              httpapi.PatchField[[]string] `json:"finalBranches"`
}

// ColumnMappingChange keeps rule 17's semantics: absent leaves the mapping, a nil Token clears it ("do not move").
type ColumnMappingChange struct {
	Present bool
	Token   *string
}

type BoardGithubSettingsChanges struct {
	Opened          ColumnMappingChange
	ReviewRequested ColumnMappingChange
	Merged          ColumnMappingChange
	FinalBranches   []string
}

func (request UpdateBoardGithubSettingsRequest) Validate() (BoardGithubSettingsChanges, error) {
	changes := BoardGithubSettingsChanges{
		Opened:          columnMappingChangeFrom(request.OpenedColumnToken),
		ReviewRequested: columnMappingChangeFrom(request.ReviewRequestedColumnToken),
		Merged:          columnMappingChangeFrom(request.MergedColumnToken),
	}
	if request.FinalBranches.Present {
		branches, valid := normalizeFinalBranches(request.FinalBranches.Value)
		if request.FinalBranches.Null || !valid {
			return BoardGithubSettingsChanges{}, httpapi.NewValidationError(map[string]string{fieldFinalBranches: "invalid"})
		}
		changes.FinalBranches = branches
	}
	return changes, nil
}

func (changes BoardGithubSettingsChanges) IsEmpty() bool {
	return !changes.Opened.Present && !changes.ReviewRequested.Present && !changes.Merged.Present && changes.FinalBranches == nil
}

func columnMappingChangeFrom(field httpapi.PatchField[string]) ColumnMappingChange {
	if !field.HasValue() {
		return ColumnMappingChange{Present: field.Present}
	}
	token := field.Value
	return ColumnMappingChange{Present: true, Token: &token}
}

// normalizeFinalBranches trims names and drops duplicates, keeping case because git branch names are case-sensitive.
func normalizeFinalBranches(submitted []string) ([]string, bool) {
	if len(submitted) > maxFinalBranches {
		return nil, false
	}
	branches := make([]string, 0, len(submitted))
	for _, branch := range submitted {
		branch = strings.TrimSpace(branch)
		if !isValidBranchName(branch) {
			return nil, false
		}
		if !slices.Contains(branches, branch) {
			branches = append(branches, branch)
		}
	}
	return branches, len(branches) > 0
}

func isValidBranchName(branch string) bool {
	return branch != "" && utf8.RuneCountInString(branch) <= maxBranchNameLength && !strings.ContainsFunc(branch, unicode.IsSpace)
}
