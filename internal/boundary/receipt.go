package boundary

import "errors"

// Receipt is the verifier's structured handoff, attached to the source issue.
// Report is the complete external-policy report, not a conductor verdict.
type Receipt struct {
	Policy         string   `json:"policy"`
	Head           string   `json:"head"`
	Verdict        string   `json:"verdict"`
	ContractSHA256 string   `json:"contract_sha256"`
	ProofsSHA256   string   `json:"proofs_sha256"`
	Author         string   `json:"author"`
	Reviewer       string   `json:"reviewer"`
	Verifier       string   `json:"verifier"`
	Gaps           []string `json:"gaps"`
	Report         string   `json:"report"`
}

func ReceiptFrom(data []byte) (Receipt, error) {
	var v Receipt
	if err := decode(data, &v, true); err != nil {
		return v, err
	}
	if !shaPattern.MatchString(v.Head) || !hashPattern.MatchString(v.ContractSHA256) || !hashPattern.MatchString(v.ProofsSHA256) || blank(v.Policy, v.Verdict, v.Author, v.Reviewer, v.Verifier, v.Report) {
		return v, errors.New("incomplete review receipt")
	}
	return v, nil
}

type Commit struct {
	SHA string `json:"sha"`
}

func CommitFrom(data []byte) (Commit, error) {
	var v Commit
	if err := decode(data, &v, true); err != nil {
		return v, err
	}
	if !shaPattern.MatchString(v.SHA) {
		return v, errors.New("invalid commit SHA")
	}
	return v, nil
}
