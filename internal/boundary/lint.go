package boundary

import "errors"

type LintPosition struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
}
type LintIssue struct {
	FromLinter string       `json:"FromLinter"`
	Pos        LintPosition `json:"Pos"`
}
type LintReport struct {
	Issues []LintIssue `json:"Issues"`
}

func LintFrom(data []byte) (LintReport, error) {
	var r LintReport
	if err := decode(data, &r, false); err != nil {
		return r, err
	}
	if len(r.Issues) == 0 {
		return r, errors.New("lint fixture produced no issues")
	}
	for _, i := range r.Issues {
		if blank(i.FromLinter, i.Pos.Filename) || i.Pos.Line < 1 {
			return r, errors.New("invalid lint issue")
		}
	}
	return r, nil
}
